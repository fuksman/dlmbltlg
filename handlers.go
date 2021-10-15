package main

import (
	"errors"

	tele "gopkg.in/tucnak/telebot.v3"
)

func provideUserToContext(next tele.HandlerFunc) tele.HandlerFunc {
	return func(tlg tele.Context) error {
		user, err := LoadUser(tlg.Sender().ID)
		if err != nil {
			return tlg.Send(err.Error())
		}

		tlg.Set("user", user)
		return next(tlg)
	}
}

func provideCompanyToContext(next tele.HandlerFunc) tele.HandlerFunc {
	return func(tlg tele.Context) error {
		user, ok := tlg.Get("user").(*User)
		if !ok {
			err := errors.New("ошибка получения информации о пользователе")
			log.Warn(err)
			return tlg.Send(err.Error())
		}
		company, err := LoadCompany(user.CompanyId)
		if err != nil {
			return tlg.Send(err.Error())
		}

		permissions := company.Permissions(user)
		tlg.Set("company", company)
		tlg.Set("permissions", permissions)

		switch permissions {
		case "admin":
			tlg.Set("menu", adminMenu)
			return next(tlg)
		case "employee":
			tlg.Set("menu", emplMenu)
			return next(tlg)
		default:
			return tlg.Send("Не могу найти компаний, к которым у тебя есть доступ", startMenu)
		}
	}
}

func ensureIsAdmin(next tele.HandlerFunc) tele.HandlerFunc {
	return func(tlg tele.Context) error {
		permissions, ok := tlg.Get("permissions").(string)
		if !ok {
			err := errors.New("ошибка получения информации о правах доступа")
			log.Warn(err)
			return tlg.Send(err.Error())
		}

		if permissions == "admin" {
			return next(tlg)
		} else {
			return tlg.Send("Команда доступна только администратору компании", emplMenu)
		}
	}
}

func SignOut() tele.HandlerFunc {
	return func(tlg tele.Context) error {
		if err := RemoveUser(tlg.Sender().ID); err != nil {
			log.Warn(err)
			return tlg.Send(err.Error())
		}

		user, ok := tlg.Get("user").(*User)
		if !ok {
			err := errors.New("ошибка получения информации о пользователе")
			log.Warn(err)
			return tlg.Send(err.Error())
		}
		if user.Admin {
			if err := RemoveCompany(user.CompanyId); err != nil {
				log.Warn(err)
				return tlg.Send(err.Error())
			}
			return tlg.Send("✅ Удалил всю информацию о связанной компании")
		}

		return tlg.Send("👋 Удалил всю информацию о пользователе, но всегда можно начать сначала", startMenu)
	}
}

func SendInvoiceMenu(tlg *tele.Context) error {
	return (*tlg).Send("Какой нужен счёт?", invoiceMenu)
}

func BuildReplyMenus() {
	startMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	unAuthMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	emplMenu = &tele.ReplyMarkup{ResizeKeyboard: true}
	adminMenu = &tele.ReplyMarkup{ResizeKeyboard: true}

	startMenu.Reply(
		startMenu.Row(startMenu.Contact("Дать номер телефона")),
	)

	unAuthMenu.Reply(
		unAuthMenu.Row(unAuthMenu.Text("Я администратор")),
		unAuthMenu.Row(unAuthMenu.Text("Разлогиниться")),
	)

	emplMenu.Reply(
		emplMenu.Row(emplMenu.Text("Баланс"), emplMenu.Text("Поездки")),
		emplMenu.Row(emplMenu.Text("Разлогиниться")),
	)

	adminMenu.Reply(
		adminMenu.Row(adminMenu.Text("Баланс"), adminMenu.Text("Поездки")),
		adminMenu.Row(adminMenu.Text("Последний счёт"), adminMenu.Text("Новый счёт")),
		adminMenu.Row(adminMenu.Text("Разлогиниться")),
	)

	invoiceMenu = &tele.ReplyMarkup{}
	btnNewInvoice3000 = invoiceMenu.Data("На 3 000 ₽", "btnNewInvoice3000")
	btnNewInvoice10000 = invoiceMenu.Data("На 10 000 ₽", "btnNewInvoice10000")
	btnNewInvoice30000 = invoiceMenu.Data("На 30 000 ₽", "btnNewInvoice30000")
	btnLastInvoice = invoiceMenu.Data("Последний", "btnLastInvoice")
	invoiceMenu.Inline(
		invoiceMenu.Row(btnNewInvoice3000, btnNewInvoice10000),
		invoiceMenu.Row(btnNewInvoice30000, btnLastInvoice),
	)
}

func ReadContext(tlg tele.Context) (*User, *Company, *tele.ReplyMarkup, error) {
	user, ok := tlg.Get("user").(*User)
	if !ok {
		err := errors.New("ошибка получения информации о пользователе")
		return nil, nil, nil, err
	}

	company, ok := tlg.Get("company").(*Company)
	if !ok {
		err := errors.New("ошибка получения информации о компании")
		return nil, nil, nil, err
	}

	menu, ok := tlg.Get("menu").(*tele.ReplyMarkup)
	if !ok {
		err := errors.New("ошибка получения информации о меню")
		return nil, nil, nil, err
	}

	return user, company, menu, nil
}
