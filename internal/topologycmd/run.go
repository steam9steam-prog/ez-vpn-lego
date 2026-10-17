// Package topologycmd implements the offline installation workbench. It never
// applies a configuration to a host or changes the running single-node daemon.
package topologycmd

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/steam9steam-prog/ez-vpn-lego/core/topology"
	topologyxray "github.com/steam9steam-prog/ez-vpn-lego/drivers/xray/topology"
	"github.com/steam9steam-prog/ez-vpn-lego/profiles"
	"github.com/steam9steam-prog/ez-vpn-lego/services/planner"
)

const maxFileSize = 16 << 20

func Run(arguments []string, stdout, stderr io.Writer) int {
	if err := run(arguments, stdout); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func run(args []string, stdout io.Writer) error {
	if len(args) == 2 && args[0] == "profiles" && args[1] == "list" {
		return output(stdout, profiles.Catalog())
	}
	if len(args) < 3 || args[0] != "topology" {
		return errors.New(usage)
	}
	switch args[1] {
	case "plan", "dns":
		if len(args) != 3 {
			return errors.New(usage)
		}
		installation, err := readInstallation(args[2])
		if err != nil {
			return err
		}
		plan, err := planner.Build(installation)
		if err != nil {
			return err
		}
		if args[1] == "plan" {
			return output(stdout, plan)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		results := []planner.DNSResult{}
		ready := true
		for _, requirement := range plan.Domains {
			result := planner.CheckDNS(ctx, nil, requirement)
			ready = ready && result.Ready
			results = append(results, result)
		}
		if err := output(stdout, results); err != nil {
			return err
		}
		if !ready {
			return errors.New("DNS is not ready; no server was changed")
		}
		return nil
	case "init", "extend":
		if len(args) != 4 {
			return errors.New(usage)
		}
		installation, err := readInstallation(args[2])
		if err != nil {
			return err
		}
		return withLock(args[3], func() error {
			var material topologyxray.Material
			if args[1] == "init" {
				if _, err := os.Lstat(args[3]); !errors.Is(err, os.ErrNotExist) {
					return errors.New("state already exists or cannot be inspected; use topology extend to preserve its keys")
				}
				material, err = topologyxray.NewMaterial(installation)
			} else {
				if err = readJSON(args[3], &material, true); err != nil {
					return err
				}
				material, err = topologyxray.ExtendMaterial(installation, material)
			}
			if err != nil {
				return err
			}
			if _, err := topologyxray.Render(installation, material); err != nil {
				return err
			}
			if err := saveJSON(args[3], material); err != nil {
				return err
			}
			return output(stdout, map[string]string{"state": args[3], "status": "saved"})
		})
	case "grant", "routes":
		if args[1] == "grant" && len(args) < 7 || args[1] == "routes" && len(args) < 6 {
			return errors.New(usage)
		}
		installation, err := readInstallation(args[2])
		if err != nil {
			return err
		}
		return withLock(args[3], func() error {
			var material topologyxray.Material
			if err := readJSON(args[3], &material, true); err != nil {
				return err
			}
			if args[1] == "grant" {
				err = topologyxray.AddDevice(installation, &material, args[4], args[5], args[6:])
			} else {
				err = topologyxray.SetDeviceRoutes(installation, &material, args[4], args[5:])
			}
			if err != nil {
				return err
			}
			if _, err := topologyxray.Render(installation, material); err != nil {
				return err
			}
			if err := saveJSON(args[3], material); err != nil {
				return err
			}
			return output(stdout, map[string]string{"device": args[4], "status": "granted"})
		})
	case "device":
		if len(args) != 5 || args[4] != "enable" && args[4] != "disable" {
			return errors.New(usage)
		}
		return withLock(args[2], func() error {
			var material topologyxray.Material
			if err := readJSON(args[2], &material, true); err != nil {
				return err
			}
			found := false
			for index := range material.Devices {
				if material.Devices[index].ID == args[3] {
					material.Devices[index].Enabled = args[4] == "enable"
					found = true
				}
			}
			if !found {
				return errors.New("device not found")
			}
			if err := saveJSON(args[2], material); err != nil {
				return err
			}
			return output(stdout, map[string]string{"device": args[3], "status": args[4] + "d", "next": "render and apply a new generation to revoke or restore imported credentials"})
		})
	case "render":
		if len(args) != 5 {
			return errors.New(usage)
		}
		installation, err := readInstallation(args[2])
		if err != nil {
			return err
		}
		var material topologyxray.Material
		if err := readJSON(args[3], &material, true); err != nil {
			return err
		}
		bundle, err := topologyxray.Render(installation, material)
		if err != nil {
			return err
		}
		if err := writeBundle(args[4], bundle, material); err != nil {
			return err
		}
		return output(stdout, map[string]any{"directory": args[4], "nodes": len(bundle.Nodes), "subscriptions": len(bundle.Subscriptions), "applied": false})
	default:
		return errors.New(usage)
	}
}

func readInstallation(path string) (topology.Installation, error) {
	var installation topology.Installation
	if err := readJSON(path, &installation, false); err != nil {
		return installation, err
	}
	return installation, installation.Validate()
}

func readJSON(path string, target any, private bool) error {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxFileSize {
		return errors.New("input must be a regular JSON file smaller than 16 MiB")
	}
	if private && info.Mode().Perm()&0o077 != 0 {
		return errors.New("private state must not be readable by other users; set its permissions to 0600")
	}
	decoder := json.NewDecoder(io.LimitReader(file, maxFileSize+1))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("input must contain exactly one JSON object")
	}
	return nil
}

func withLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return errors.New("another command is changing this state")
	}
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN) }()
	return fn()
}

func saveJSON(path string, value any) error {
	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return errors.New("refusing to replace a symlink or non-regular state file")
	}
	content, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".lego-state-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err := file.Write(append(content, '\n')); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func writeBundle(destination string, bundle topologyxray.Bundle, material topologyxray.Material) error {
	if _, err := os.Lstat(destination); !errors.Is(err, os.ErrNotExist) {
		return errors.New("output already exists or cannot be inspected; use a new directory for each generation")
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	stage, err := os.MkdirTemp(parent, ".lego-generation-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := os.Mkdir(filepath.Join(stage, "nodes"), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(stage, "subscriptions"), 0o700); err != nil {
		return err
	}
	hashes := map[string]string{}
	write := func(name string, data []byte) error {
		if err := os.WriteFile(filepath.Join(stage, name), data, 0o600); err != nil {
			return err
		}
		digest := sha256.Sum256(data)
		hashes[name] = hex.EncodeToString(digest[:])
		return nil
	}
	for nodeID, data := range bundle.Nodes {
		if err := write("nodes/"+nodeID+".json", data); err != nil {
			return err
		}
	}
	for _, device := range material.Devices {
		uris, exists := bundle.Subscriptions[device.ID]
		if !exists {
			continue
		}
		body := base64.StdEncoding.EncodeToString([]byte(strings.Join(uris, "\n")+"\n")) + "\n"
		if err := write("subscriptions/"+device.SubscriptionToken, []byte(body)); err != nil {
			return err
		}
	}
	manifest, err := json.MarshalIndent(map[string]any{"schema_version": 1, "applied": false, "sha256": hashes}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "manifest.json"), append(manifest, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(stage, destination)
}

func output(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

const usage = `usage:
  lego-vpnctl profiles list
  lego-vpnctl topology plan INSTALLATION.json
  lego-vpnctl topology dns INSTALLATION.json
  lego-vpnctl topology init INSTALLATION.json STATE.json
  lego-vpnctl topology extend INSTALLATION.json STATE.json
  lego-vpnctl topology grant INSTALLATION.json STATE.json DEVICE_ID NAME ROUTE_ID...
  lego-vpnctl topology routes INSTALLATION.json STATE.json DEVICE_ID ROUTE_ID...
  lego-vpnctl topology device STATE.json DEVICE_ID {enable|disable}
  lego-vpnctl topology render INSTALLATION.json STATE.json OUTPUT_DIRECTORY

These commands prepare local files. They do not apply a configuration to servers.`
