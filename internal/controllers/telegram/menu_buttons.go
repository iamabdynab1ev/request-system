package telegram

const (
	menuMyTasksButton   = "📋 Мои заявки"
	menuAssignedButton  = "👨‍💼 Назначены мне"
	menuTodayButton     = "⏰ На сегодня"
	menuOverdueButton   = "🔴 Просроченные"
	menuStatsButton     = "📊 Статистика"
	menuSearchButton    = "🔍 Поиск"
	menuStatusButton    = "🔐 Статус"
	menuHelpButton      = "📖 Справка"
	menuMainButton      = "🏠 Главное меню"
	menuBackButton      = "◀️ Назад"
	unlinkButton        = "🔓 Отвязать Telegram"
	confirmUnlinkButton = "✅ Да, отвязать"
	cancelButton        = "↩️ Отмена"
)

func isTelegramMenuButton(text string) bool {
	switch text {
	case menuMyTasksButton,
		menuAssignedButton,
		menuTodayButton,
		menuOverdueButton,
		menuStatsButton,
		menuSearchButton,
		menuStatusButton,
		menuHelpButton,
		menuMainButton:
		return true
	default:
		return false
	}
}
