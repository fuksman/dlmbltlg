package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"errors"
	log "github.com/sirupsen/logrus"
	"strconv"

	"cloud.google.com/go/firestore"
	"github.com/fuksman/delimobil"
	tele "gopkg.in/tucnak/telebot.v3"
)

var (
	ctx                                                                       context.Context
	client                                                                    *firestore.Client
	invoiceMenu                                                               *tele.ReplyMarkup
	btnNewInvoice3000, btnNewInvoice10000, btnNewInvoice30000, btnLastInvoice tele.Btn
	authError                                                                 error
)

func init() {
	log.SetLevel(log.WarnLevel)
	log.SetReportCaller(true)

	invoiceMenu = &tele.ReplyMarkup{}
	btnNewInvoice3000 = invoiceMenu.Data("На 3 000 ₽", "btnNewInvoice3000")
	btnNewInvoice10000 = invoiceMenu.Data("На 10 000 ₽", "btnNewInvoice10000")
	btnNewInvoice30000 = invoiceMenu.Data("На 30 000 ₽", "btnNewInvoice30000")
	btnLastInvoice = invoiceMenu.Data("Последний", "btnLastInvoice")
	invoiceMenu.Inline(
		invoiceMenu.Row(btnNewInvoice3000, btnNewInvoice10000),
		invoiceMenu.Row(btnNewInvoice30000, btnLastInvoice),
	)
	authError = errors.New("🤔 Кажется, мы ещё не знакомы. Отправь команду\n/auth login password")

	ctx = context.Background()

	var err error
	client, err = firestore.NewClient(ctx, "dlmbltlg")
	if err != nil {
		log.Fatal("Can't run firestore client", err)
		return
	}
}

func main() {
	tlgDoc, err := client.Collection("Secrets").Doc("Telegram").Get(ctx)
	if err != nil {
		log.Fatal("Can't get telegram key from firestore", err)
		return
	}

	tlgKey, err := tlgDoc.DataAt("key")
	if err != nil {
		log.Fatal("Can't get telegram key from firestore", err)
		return
	}

	b, err := tele.NewBot(tele.Settings{
		Token:  tlgKey.(string),
		Poller: &tele.LongPoller{Timeout: 10 * 1e9},
	})
	if err != nil {
		log.Fatal("Can't run telegram bot", err)
		return
	}

	b.Handle("/start", func(c tele.Context) error {
		mes := "Привет! Вот мой список команд:\n/auth — Авторизоваться\n/balance — Показать текущий баланс\n/rides — Показать последние поездки\n/newinvoice — Создать новый счёт\n/lastinvoice — Получить последний счёт"
		return c.Send(mes)
	})

	b.Handle("/auth", func(tlg tele.Context) error {
		removed := "\nДля безопасности я удалил сообщение с логином и паролем."
		if len(tlg.Args()) != 2 {
			tlg.Delete()
			return tlg.Send("Что-то не так.\nПравильное использование:\n /auth login password" + removed)
		}

		user, err := SaveUser(tlg.Args()[0], tlg.Args()[1], tlg.Sender().ID)
		if err != nil {
			tlg.Delete()
			return tlg.Send(err.Error() + removed)
		}

		tlg.Delete()
		return tlg.Send("Привет, " + user.Name() + "!\nВсё настроено, можем работать." + removed)
	})

	b.Handle("/balance", func(tlg tele.Context) error {
		user, err := UserData(tlg.Sender().ID)
		if err != nil {
			return tlg.Send(err.Error())
		}
		if err := user.SetCompanyInfo(); err != nil {
			return tlg.Send(err.Error())
		}

		balanceLimit := float64(1000)
		mes := user.Name() + " (" + user.Org.Name + ")\n" +
			"Текущий баланс: " + strconv.FormatFloat(user.Org.Balance, 'f', 2, 64) + " ₽"

		if user.IsBalanceOK(balanceLimit) {
			return tlg.Send(mes)
		}
		mes += "\n\n🚨 Баланс меньше " + strconv.FormatFloat(balanceLimit, 'f', 2, 64) + " ₽!\nНадо пополнять."
		tlg.Send(mes)

		return SendInvoiceMenu(&tlg)
	})

	b.Handle("/rides", func(tlg tele.Context) error {
		user, err := UserData(tlg.Sender().ID)
		if err != nil {
			return tlg.Send(err.Error())
		}
		if err := user.SetCompanyInfo(); err != nil {
			return tlg.Send(err.Error())
		}
		if err := user.SetRidesInfo(10, 1); err != nil {
			return tlg.Send(err.Error())
		}
		mes := "Последние поездки:\n" + user.Org.Rides.String()
		return tlg.Send(mes)
	})

	b.Handle("/lastinvoice", func(tlg tele.Context) error {
		return Invoice(&tlg)
	})

	b.Handle("/newinvoice", func(tlg tele.Context) error {
		return SendInvoiceMenu(&tlg)
	})

	b.Handle(&btnNewInvoice3000, func(tlg tele.Context) error {
		Invoice(&tlg, 3000)
		return tlg.Respond()
	})

	b.Handle(&btnNewInvoice10000, func(tlg tele.Context) error {
		Invoice(&tlg, 10000)
		return tlg.Respond()
	})

	b.Handle(&btnNewInvoice30000, func(tlg tele.Context) error {
		Invoice(&tlg, 30000)
		return tlg.Respond()
	})

	b.Handle(&btnLastInvoice, func(tlg tele.Context) error {
		Invoice(&tlg)
		return tlg.Respond()
	})

	b.Start()
}

func SendInvoiceMenu(tlg *tele.Context) error {
	return (*tlg).Send("Какой нужен счёт?", invoiceMenu)
}

func Invoice(tlg *tele.Context, amount ...float64) error {
	senderLogger := log.WithField("id", (*tlg).Sender().ID)
	user, err := UserData((*tlg).Sender().ID)
	if err != nil {
		(*tlg).Send(err.Error())
		return err
	}
	var invoice *delimobil.File
	if amount != nil {
		senderLogger.Trace("Creating new invoice...")
		invoice, err = user.CreateInvoice(amount[0])
	} else {
		senderLogger.Trace("Retrieving last invoice...")
		invoice, err = user.LastInvoice()
	}
	if err != nil {
		senderLogger.Warn("Can't create/retrieve invoice.")
		(*tlg).Send(err.Error())
		return err
	}
	senderLogger.Info("Created/recieved invoice.")
	doc := &tele.Document{File: tele.FromReader(invoice.Data)}
	doc.FileName = invoice.FileName
	doc.MIME = invoice.MIME
	return (*tlg).Send(doc)
}

func UserData(id int64) (user *delimobil.User, err error) {
	senderLogger := log.WithField("id", id)
	senderLogger.Trace("Getting user data...")
	user, err = LoadUser(id)
	if err != nil {
		senderLogger.Warn(err)
		return nil, err
	}

	if user.IsValid() {
		senderLogger.Trace("Token is valid.")
		senderLogger.Info("Returned user data!")
		return user, nil
	}

	senderLogger.Info("Token is not valid.")
	senderLogger.Trace("Updating token...")
	user, err = SaveUser(user.Login, user.Password, id)
	if err != nil {
		return nil, err
	}

	senderLogger.Info("Returned user data!")
	return user, nil
}

func LoadUser(id int64) (user *delimobil.User, err error) {
	senderLogger := log.WithField("id", id)
	senderLogger.Trace("Loading user...")
	userDocRef := client.Collection("Users").Doc(strconv.FormatInt(id, 10))

	userDoc, err := userDocRef.Get(ctx)
	if err != nil {
		senderLogger.Warn(err)
		return nil, authError
	}

	userGob, err := userDoc.DataAt("gob")
	if err != nil {
		senderLogger.Warn(err)
		return nil, authError
	}

	by, err := base64.StdEncoding.DecodeString(userGob.(string))
	if err != nil {
		senderLogger.Warn(err)
		return nil, err
	}
	buf := bytes.Buffer{}
	buf.Write(by)
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&user)
	if err != nil {
		senderLogger.Warn(err)
		return nil, err
	}

	senderLogger.Info("Loaded!")
	return user, nil
}

func SaveUser(login string, password string, id int64) (user *delimobil.User, err error) {
	senderLogger := log.WithField("id", id)
	senderLogger.Trace("Updating user token...")
	userDocRef := client.Collection("Users").Doc(strconv.FormatInt(id, 10))

	user = new(delimobil.User)
	if err := user.Auth(login, password); err != nil {
		senderLogger.Warn(err)
		return nil, authError
	}
	senderLogger.Info("Token updated.")

	senderLogger.Trace("Saving user...")
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(user); err != nil {
		senderLogger.Warn(err)
		return nil, err
	}

	if _, err := userDocRef.Set(ctx, map[string]interface{}{"gob": base64.StdEncoding.EncodeToString(buf.Bytes())}, firestore.MergeAll); err != nil {
		senderLogger.Warn(err)
		return nil, err
	}

	senderLogger.Info("Saved!")
	return user, nil
}
