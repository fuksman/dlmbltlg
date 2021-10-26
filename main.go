package main

import (
	"context"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/sirupsen/logrus"
	tele "gopkg.in/tucnak/telebot.v3"
)

var (
	log                                                                       = logrus.New()
	appConfig                                                                 AppConfig
	ctx                                                                       context.Context
	client                                                                    *firestore.Client
	startMenu, unAuthMenu, emplMenu, adminMenu, invoiceMenu                   *tele.ReplyMarkup
	btnNewInvoice3000, btnNewInvoice10000, btnNewInvoice30000, btnLastInvoice tele.Btn
)

func init() {
	if err := appConfig.LoadConfiguration(); err != nil {
		log.Fatal(err)
		return
	}
	switch appConfig.Environment {
	case "prod":
		log.SetLevel(logrus.WarnLevel)
	case "test":
		log.SetLevel(logrus.TraceLevel)
		log.SetReportCaller(true)
		log.Info("Using 'test' environment")
	default:
		log.Fatal("'environment' should be 'test' or 'prod', but now it is ", appConfig.Environment)
		return
	}

	BuildReplyMenus()

	ctx = context.Background()

	var err error
	client, err = firestore.NewClient(ctx, appConfig.ProjectID)
	if err != nil {
		log.Fatal("Can't run firestore client", err)
		return
	}
}

func main() {
	b, err := tele.NewBot(tele.Settings{
		Token:  appConfig.TelegramToken,
		Poller: &tele.LongPoller{Timeout: 10 * 1e9},
	})
	if err != nil {
		log.Fatal("Can't run telegram bot", err)
		return
	}

	b.Handle("/start", func(tlg tele.Context) error {
		return tlg.Send("Для работы мне нужен твой телефонный номер.", startMenu)
	})

	b.Handle(tele.OnContact, func(tlg tele.Context) error {
		if tlg.Message().Contact.UserID != int(tlg.Sender().ID) {
			return tlg.Send("😡 Меня не обманешь, можно прислать только свой контакт", startMenu)
		}

		user := User{Id: tlg.Sender().ID, Phone: tlg.Message().Contact.PhoneNumber}

		if err := user.SaveUser(); err != nil {
			return tlg.Send("Не могу сохранить информацию 😔", startMenu)
		}

		company, err := user.FindCompany()
		if err != nil {
			return tlg.Send("Произошла ошибка при поиске подключенных компаний 😔", startMenu)
		}

		if company != nil {
			company.SetInfo()
			return tlg.Send("👋 Привет!\nТеперь у тебя есть доступ к компании "+company.Info.Name, emplMenu)
		}

		return tlg.Send("Сохранил, но не могу найти ни одну подходящую компанию.\nАдминистратор должен подключить компанию к боту или добавить тебя в список сотрудников.", unAuthMenu)
	})

	b.Handle("Я администратор", func(tlg tele.Context) error {
		return tlg.Send("Отправь команду\n`/auth login password`\n где `login` и `password` — реквизиты доступа к личному кабинету Делимобиль", tele.ModeMarkdownV2)
	})

	// User-related handlers
	userBot := b.Group()
	userBot.Use(provideUserToContext)

	userBot.Handle("/auth", func(tlg tele.Context) error {
		removed := "\nДля безопасности я удалил сообщение с логином и паролем."
		if len(tlg.Args()) != 2 {
			tlg.Delete()
			return tlg.Send("Что-то не так.\nПравильное использование:\n /auth login password" + removed)
		}

		userLogger := log.WithField("userId", tlg.Sender().ID)
		userLogger.Trace("Updating user token...")

		company := NewCompany(tlg.Args()[0], tlg.Args()[1])
		if err := company.Authenticate(); err != nil {
			userLogger.Warn(err)
			return tlg.Send(err.Error())
		}
		userLogger.Info("Token updated.")

		if err := company.SaveCompany(); err != nil {
			tlg.Delete()
			return tlg.Send(err.Error() + removed)
		}

		user, ok := tlg.Get("user").(*User)
		if !ok {
			return tlg.Send("Не могу получить информацию о пользователе" + removed)
		}
		user.CompanyId = company.Id
		user.Admin = true
		user.SetLastBalance(company)
		if err := user.SaveUser(); err != nil {
			tlg.Delete()
			return tlg.Send("Авторизация прошла, но с ошибкой:\n"+err.Error()+removed, startMenu)
		}

		if err := company.SetInfo(); err != nil {
			tlg.Delete()
			return tlg.Send("Авторизация прошла, но с ошибкой:\n"+err.Error()+removed, startMenu)
		}

		tlg.Delete()
		return tlg.Send("Предоставлен доступ к компании "+company.Info.Name+"!\nВсё настроено, можем работать."+removed, adminMenu)
	})

	userBot.Handle("Разлогиниться", SignOut())
	userBot.Handle("/stop", SignOut())

	// Company-related handlers
	companyBot := b.Group()
	companyBot.Use(provideUserToContext)
	companyBot.Use(provideCompanyToContext)

	companyBot.Handle("Баланс", func(tlg tele.Context) error {
		user, company, menu, err := ReadContext(tlg)
		if err != nil {
			return tlg.Send(err.Error(), startMenu)
		}

		if err := company.SetInfo(); err != nil {
			return tlg.Send(err.Error(), menu)
		}

		mes := company.Info.Name + "\n" +
			"Текущий баланс: " + strconv.FormatFloat(company.Info.Balance, 'f', 2, 64) + " ₽"

		err = tlg.Send(mes, menu)
		if err == nil {
			user.SetLastBalance(company)
			user.SaveUser()
		}

		return err
	})

	companyBot.Handle("Поездки", func(tlg tele.Context) error {
		_, company, menu, err := ReadContext(tlg)
		if err != nil {
			return tlg.Send(err.Error(), startMenu)
		}

		if err := company.SetInfo(); err != nil {
			return tlg.Send(err.Error(), menu)
		}
		if err := company.SetRides(10, 1); err != nil {
			return tlg.Send(err.Error(), menu)
		}
		mes := "Последние поездки:\n" + company.Rides.String()

		return tlg.Send(mes, menu)
	})

	// Admin-only handlers
	adminBot := b.Group()
	adminBot.Use(provideUserToContext)
	adminBot.Use(provideCompanyToContext)
	adminBot.Use(ensureIsAdmin)

	adminBot.Handle("Последний счёт", func(tlg tele.Context) error {
		return Invoice(&tlg)
	})

	adminBot.Handle("Новый счёт", func(tlg tele.Context) error {
		return SendInvoiceMenu(&tlg)
	})

	adminBot.Handle("Последние закрывающие", func(tlg tele.Context) error {
		return LastClosingDocuments(&tlg)
	})

	adminBot.Handle(&btnNewInvoice3000, func(tlg tele.Context) error {
		Invoice(&tlg, 3000)
		return tlg.Respond()
	})

	adminBot.Handle(&btnNewInvoice10000, func(tlg tele.Context) error {
		Invoice(&tlg, 10000)
		return tlg.Respond()
	})

	adminBot.Handle(&btnNewInvoice30000, func(tlg tele.Context) error {
		Invoice(&tlg, 30000)
		return tlg.Respond()
	})

	adminBot.Handle(&btnLastInvoice, func(tlg tele.Context) error {
		Invoice(&tlg)
		return tlg.Respond()
	})

	log.Trace("Starting balance change notifyer...")
	ticker := time.NewTicker(time.Duration(appConfig.CheckDelay) * time.Second)
	defer ticker.Stop()
	go func() {
		for range ticker.C {
			NotifyAboutBalanceChange(b)
		}
	}()

	log.Trace("Starting bot...")
	b.Start()
}
