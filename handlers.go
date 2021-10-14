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

func checkIsActive(next tele.HandlerFunc) tele.HandlerFunc {
	return func(tlg tele.Context) error {
		user, ok := tlg.Get("user").(*User)
		if !ok {
			err := errors.New("ошибка получения информации о пользователе")
			log.Warn(err)
			return tlg.Send(err.Error())
		}

		if user.Admin {
			tlg.Set("menu", adminMenu)
			return next(tlg)
		}

		if user.IsActive() {
			tlg.Set("menu", emplMenu)
			return next(tlg)
		}

		return tlg.Send("Не могу найти компаний, к которым у тебя есть доступ", startMenu)
	}
}

func ensureIsAdmin(next tele.HandlerFunc) tele.HandlerFunc {
	return func(tlg tele.Context) error {
		user, ok := tlg.Get("user").(*User)
		if !ok {
			err := errors.New("ошибка получения информации о пользователе")
			log.Warn(err)
			return tlg.Send(err.Error())
		}

		if user.Admin {
			return next(tlg)
		} else {
			return tlg.Send("Команда доступна только администратору компании", emplMenu)
		}
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

		tlg.Set("company", company)

		return next(tlg)
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
