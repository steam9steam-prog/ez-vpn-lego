package telegrambot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/steam9steam-prog/ez-vpn-lego/adapters/controlclient"
	"github.com/steam9steam-prog/ez-vpn-lego/internal/id"
)

type App struct {
	client  *controlclient.Client
	mu      sync.Mutex
	waiting map[int64]bool
}

func New(client *controlclient.Client) *App {
	return &App{client: client, waiting: make(map[int64]bool)}
}

func (a *App) Options() []bot.Option {
	return []bot.Option{
		bot.WithDefaultHandler(a.handleMessage),
		bot.WithCallbackQueryDataHandler("menu:", bot.MatchTypePrefix, a.handleCallback),
	}
}

func (a *App) handleMessage(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil || update.Message.Chat.Type != models.ChatTypePrivate {
		return
	}
	subject := strconv.FormatInt(update.Message.From.ID, 10)
	text := strings.TrimSpace(update.Message.Text)
	if strings.HasPrefix(text, "/start") {
		a.start(ctx, b, update.Message.Chat.ID, subject, strings.TrimSpace(strings.TrimPrefix(text, "/start")))
		return
	}
	admin, err := a.client.ResolveTelegram(ctx, subject)
	if err != nil {
		a.send(ctx, b, update.Message.Chat.ID, "Этот Telegram ещё не привязан. Откройте одноразовую ссылку, показанную установщиком.", nil)
		return
	}
	if text == "/new" || a.takeWaiting(update.Message.Chat.ID) {
		if text == "/new" {
			a.setWaiting(update.Message.Chat.ID)
			a.send(ctx, b, update.Message.Chat.ID, "Как назвать новый доступ? Например: iPhone или Семья", nil)
			return
		}
		a.createAccess(ctx, b, update.Message.Chat.ID, admin.ID, text)
		return
	}
	a.menu(ctx, b, update.Message.Chat.ID, "Управление вашим VPN")
}

func (a *App) start(ctx context.Context, b *bot.Bot, chatID int64, subject, token string) {
	_, err := a.client.ResolveTelegram(ctx, subject)
	if err == nil {
		a.menu(ctx, b, chatID, "Готово — вы уже авторизованы.")
		return
	}
	if token == "" {
		a.send(ctx, b, chatID, "Нужна одноразовая ссылка привязки из установщика.", nil)
		return
	}
	if _, err := a.client.ClaimTelegramPairing(ctx, token, subject); err != nil {
		a.send(ctx, b, chatID, "Ссылка недействительна или уже использована. Создайте новую через lego-vpnctl telegram pairing.", nil)
		return
	}
	a.menu(ctx, b, chatID, "Аккаунт привязан. Теперь сервером можно управлять здесь.")
}

func (a *App) handleCallback(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.CallbackQuery == nil || update.CallbackQuery.Message.Message == nil {
		return
	}
	_ = a.answer(ctx, b, update.CallbackQuery.ID)
	chatID := update.CallbackQuery.Message.Message.Chat.ID
	subject := strconv.FormatInt(update.CallbackQuery.From.ID, 10)
	if _, err := a.client.ResolveTelegram(ctx, subject); err != nil {
		a.send(ctx, b, chatID, "Доступ запрещён.", nil)
		return
	}
	switch update.CallbackQuery.Data {
	case "menu:new":
		a.setWaiting(chatID)
		a.send(ctx, b, chatID, "Напишите название нового доступа.", nil)
	case "menu:status":
		health, err := a.client.Health(ctx)
		if err != nil {
			a.send(ctx, b, chatID, "Сервис временно недоступен.", nil)
			return
		}
		a.menu(ctx, b, chatID, fmt.Sprintf("Сервер работает · версия %s", health.Version))
	case "menu:home":
		a.menu(ctx, b, chatID, "Управление вашим VPN")
	}
}

func (a *App) createAccess(ctx context.Context, b *bot.Bot, chatID int64, adminID, name string) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > 80 {
		a.send(ctx, b, chatID, "Название должно содержать от 1 до 80 символов.", nil)
		return
	}
	key, err := id.NewUUID()
	if err != nil {
		a.send(ctx, b, chatID, "Не удалось создать доступ.", nil)
		return
	}
	result, err := a.client.CreateAccess(ctx, name, key, adminID)
	if err != nil {
		a.send(ctx, b, chatID, "Не удалось применить конфигурацию. Старый доступ продолжает работать.", nil)
		return
	}
	a.send(ctx, b, chatID, "Доступ «"+name+"» создан. Нажмите на ссылку ниже, чтобы импортировать его в клиент:\n\n"+result.URI, homeKeyboard())
}

func (a *App) menu(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	a.send(ctx, b, chatID, text, &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{
		{Text: "➕ Новый доступ", CallbackData: "menu:new"},
	}, {
		{Text: "🟢 Статус", CallbackData: "menu:status"},
	}}})
}

func homeKeyboard() *models.InlineKeyboardMarkup {
	return &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{{{Text: "← В меню", CallbackData: "menu:home"}}}}
}

func (a *App) send(ctx context.Context, b *bot.Bot, chatID int64, text string, markup models.ReplyMarkup) {
	disabled := true
	_, _ = b.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text, ReplyMarkup: markup, LinkPreviewOptions: &models.LinkPreviewOptions{IsDisabled: &disabled}})
}

func (a *App) answer(ctx context.Context, b *bot.Bot, id string) error {
	_, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{CallbackQueryID: id})
	return err
}

func (a *App) setWaiting(chatID int64) { a.mu.Lock(); defer a.mu.Unlock(); a.waiting[chatID] = true }
func (a *App) takeWaiting(chatID int64) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	waiting := a.waiting[chatID]
	delete(a.waiting, chatID)
	return waiting
}
