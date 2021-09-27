package main

import (
	"bytes"
	"context"
	"dlmbltlg/pkg/delimobil"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"log"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/option"
	tb "gopkg.in/tucnak/telebot.v2"
)

type ctxKey string

func main() {
	ctx := context.Background()

	client, err := firestore.NewClient(ctx, "dlmbltlg", option.WithCredentialsFile("secrets/firestore.json"))
	if err != nil {
		log.Fatal(err)
		return
	}

	tlgDoc, err := client.Collection("Secrets").Doc("Telegram").Get(ctx)
	if err != nil {
		log.Fatal("can't get telegram key from firestore", err)
		return
	}

	tlgKey, err := tlgDoc.DataAt("key")
	if err != nil {
		log.Fatal("can't get telegram key from firestore", err)
		return
	}

	b, err := tb.NewBot(tb.Settings{
		Token:  tlgKey.(string),
		Poller: &tb.LongPoller{Timeout: 10 * 1e9},
	})
	if err != nil {
		log.Fatal(err)
		return
	}

	selector := &tb.ReplyMarkup{}
	btnNewInvoice3000 := selector.Data("На 3 000 ₽", "btnNewInvoice3000")
	btnNewInvoice10000 := selector.Data("На 10 000 ₽", "btnNewInvoice10000")
	btnNewInvoice30000 := selector.Data("На 30 000 ₽", "btnNewInvoice30000")
	btnLastInvoice := selector.Data("Последний", "btnLastInvoice")
	selector.Inline(
		selector.Row(btnNewInvoice3000, btnNewInvoice10000),
		selector.Row(btnNewInvoice30000, btnLastInvoice),
	)

	ctx = context.WithValue(ctx, ctxKey("firestore"), client)
	ctx = context.WithValue(ctx, ctxKey("bot"), b)
	ctx = context.WithValue(ctx, ctxKey("invoiceMenu"), selector)

	b.Handle("/start", func(m *tb.Message) {
		mes := "Привет! Вот мой список команд:\n/auth — Авторизоваться\n/balance — Показать текущий баланс\n/rides — Показать последние поездки\n/newinvoice — Создать новый счёт\n/lastinvoice — Получить последний счёт"
		b.Send(m.Sender, mes)
	})

	b.Handle("/auth", func(m *tb.Message) {
		creds := strings.Fields(m.Payload)
		removed := "\nДля безопасности я удалил сообщение с логином и паролем."
		if len(creds) != 2 {
			b.Delete(m)
			b.Send(m.Sender, "Что-то не так.\nПравильное использование:\n /auth login password"+removed)
			return
		}

		user := new(delimobil.User)
		if err := user.Auth(creds[0], creds[1]); err != nil {
			log.Print(err)
			b.Delete(m)
			b.Send(m.Sender, err.Error()+removed)
			return
		}
		go SaveUser(&ctx, user, m.Sender.ID)

		b.Delete(m)
		b.Send(m.Sender, "Привет, "+user.Name()+"!\nВсё настроено, можем работать."+removed)
	})

	b.Handle("/balance", func(m *tb.Message) {
		user, err := UserCredentials(&ctx, m.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(m.Sender, err.Error())
			return
		}
		if err := user.SetCompanyInfo(); err != nil {
			log.Print(err)
			b.Send(m.Sender, err.Error())
			return
		}
		mes := user.Name() + " (" + user.Org.Name + ")\n" +
			"Текущий баланс: " + strconv.FormatFloat(user.Org.Balance, 'f', 2, 64) + " ₽"
		b.Send(m.Sender, mes)
		NotifyAboutBalance(&ctx, user, m.Sender)
	})

	b.Handle("/rides", func(m *tb.Message) {
		user, err := UserCredentials(&ctx, m.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(m.Sender, err.Error())
			return
		}
		if err := user.SetCompanyInfo(); err != nil {
			log.Print(err)
			b.Send(m.Sender, err.Error())
			return
		}
		if err := user.SetRidesInfo(10, 1); err != nil {
			log.Print(err)
			b.Send(m.Sender, err.Error())
			return
		}
		mes := "Последние поездки:\n" + user.Org.Rides.String()
		b.Send(m.Sender, mes)
		NotifyAboutBalance(&ctx, user, m.Sender)
	})

	b.Handle("/lastinvoice", func(m *tb.Message) {
		Invoice(&ctx, m.Sender)
	})

	b.Handle("/newinvoice", func(m *tb.Message) {
		SendInvoiceMenu(&ctx, m.Sender)
	})

	b.Handle(&btnNewInvoice3000, func(c *tb.Callback) {
		Invoice(&ctx, c.Sender, 3000)
		b.Respond(c)
	})

	b.Handle(&btnNewInvoice10000, func(c *tb.Callback) {
		Invoice(&ctx, c.Sender, 10000)
		b.Respond(c)
	})

	b.Handle(&btnNewInvoice30000, func(c *tb.Callback) {
		Invoice(&ctx, c.Sender, 30000)
		b.Respond(c)
	})

	b.Handle(&btnLastInvoice, func(c *tb.Callback) {
		Invoice(&ctx, c.Sender)
		b.Respond(c)
	})

	b.Start()
}

func NotifyAboutBalance(ctx *context.Context, user *delimobil.User, sender *tb.User) {
	bot := (*ctx).Value(ctxKey("bot")).(*tb.Bot)
	balanceLimit := float64(1000)

	if user.IsBalanceOK(balanceLimit) {
		return
	}
	mes := "🚨 Баланс меньше " + strconv.FormatFloat(balanceLimit, 'f', 2, 64) + " ₽!\nНадо пополнять."
	bot.Send(sender, mes)
	SendInvoiceMenu(ctx, sender)
}

func SendInvoiceMenu(ctx *context.Context, sender *tb.User) {
	bot := (*ctx).Value(ctxKey("bot")).(*tb.Bot)
	menu := (*ctx).Value(ctxKey("invoiceMenu")).(*tb.ReplyMarkup)
	bot.Send(sender, "Какой нужен счёт?", menu)
}

func Invoice(ctx *context.Context, sender *tb.User, amount ...float64) {
	bot := (*ctx).Value(ctxKey("bot")).(*tb.Bot)
	user, err := UserCredentials(ctx, sender.ID)
	if err != nil {
		log.Print(err)
		bot.Send(sender, err.Error())
		return
	}
	var invoice *delimobil.File
	if amount != nil {
		invoice, err = user.CreateInvoice(amount[0])
	} else {
		invoice, err = user.LastInvoice()
	}
	if err != nil {
		log.Print(err)
		bot.Send(sender, err.Error())
		return
	}
	doc := &tb.Document{File: tb.FromReader(invoice.Data)}
	doc.FileName = invoice.FileName
	doc.MIME = invoice.MIME
	bot.Send(sender, doc)
}

func UserCredentials(ctx *context.Context, ID int) (user *delimobil.User, err error) {
	user, err = LoadUser(ctx, ID)
	if err != nil {
		return nil, err
	}

	if user.IsValid() {
		return user, nil
	}

	if err := user.Auth(user.Login, user.Password); err != nil {
		return nil, err
	}
	go SaveUser(ctx, user, ID)

	return user, nil
}

func LoadUser(ctx *context.Context, ID int) (user *delimobil.User, err error) {
	client := (*ctx).Value(ctxKey("firestore")).(*firestore.Client)
	userDocRef := client.Collection("Users").Doc(strconv.Itoa(ID))
	authError := errors.New("🤔 Кажется, мы ещё не знакомы. Отправь команду\n/auth login password")

	userDoc, err := userDocRef.Get(*ctx)
	if err != nil {
		return nil, authError
	}

	userGob, err := userDoc.DataAt("gob")
	if err != nil {
		return nil, authError
	}

	by, err := base64.StdEncoding.DecodeString(userGob.(string))
	if err != nil {
		return nil, err
	}
	buf := bytes.Buffer{}
	buf.Write(by)
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&user)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func SaveUser(ctx *context.Context, user *delimobil.User, ID int) error {
	client := (*ctx).Value(ctxKey("firestore")).(*firestore.Client)
	userDocRef := client.Collection("Users").Doc(strconv.Itoa(ID))

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(user); err != nil {
		log.Print(err)
		return err
	}

	if _, err := userDocRef.Set(*ctx, map[string]interface{}{"gob": base64.StdEncoding.EncodeToString(buf.Bytes())}); err != nil {
		log.Print(err)
		return err
	}

	return nil
}
