package main

import (
	"bytes"
	"cloud.google.com/go/firestore"
	"context"
	"dlmbltlg/pkg/delimobil"
	"encoding/base64"
	"encoding/gob"
	"errors"
	"google.golang.org/api/option"
	tb "gopkg.in/tucnak/telebot.v2"
	"log"
	"strconv"
	"strings"
)

func main() {
	ctx := context.Background()

	client, err := firestore.NewClient(ctx, "dlmbltlg", option.WithCredentialsFile("dlmbltlg-fd1662fc1892.json"))
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
		user, err := SetUserCredentials(client, &ctx, m.Sender.ID, creds[0], creds[1])
		if err != nil {
			log.Print(err)
			b.Delete(m)
			b.Send(m.Sender, err.Error()+removed)
			return
		}
		b.Delete(m)
		b.Send(m.Sender, "Привет, "+user.Name()+"!\nВсё настроено, можем работать."+removed)
	})

	b.Handle("/balance", func(m *tb.Message) {
		user, err := UserCredentials(client, &ctx, m.Sender.ID)
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
		NotifyAboutBalance(user, b, m.Sender, selector)
	})

	b.Handle("/rides", func(m *tb.Message) {
		user, err := UserCredentials(client, &ctx, m.Sender.ID)
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
		NotifyAboutBalance(user, b, m.Sender, selector)
	})

	b.Handle("/lastinvoice", func(m *tb.Message) {
		user, err := UserCredentials(client, &ctx, m.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(m.Sender, err.Error())
			return
		}
		Invoice(user, b, m.Sender)
	})

	b.Handle("/newinvoice", func(m *tb.Message) {
		SendInvoiceMenu(b, m.Sender, selector)
	})

	b.Handle(&btnNewInvoice3000, func(c *tb.Callback) {
		user, err := UserCredentials(client, &ctx, c.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(c.Sender, err.Error())
			return
		}
		Invoice(user, b, c.Sender, 3000)
		b.Respond(c)
	})

	b.Handle(&btnNewInvoice10000, func(c *tb.Callback) {
		user, err := UserCredentials(client, &ctx, c.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(c.Sender, err.Error())
			return
		}
		Invoice(user, b, c.Sender, 10000)
		b.Respond(c)
	})

	b.Handle(&btnNewInvoice30000, func(c *tb.Callback) {
		user, err := UserCredentials(client, &ctx, c.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(c.Sender, err.Error())
			return
		}
		Invoice(user, b, c.Sender, 30000)
		b.Respond(c)
	})

	b.Handle(&btnLastInvoice, func(c *tb.Callback) {
		user, err := UserCredentials(client, &ctx, c.Sender.ID)
		if err != nil {
			log.Print(err)
			b.Send(c.Sender, err.Error())
			return
		}
		Invoice(user, b, c.Sender)
		b.Respond(c)
	})

	b.Start()
}

func NotifyAboutBalance(user *delimobil.User, b *tb.Bot, sender *tb.User, menu *tb.ReplyMarkup) {
	balanceLimit := float64(1000)
	if user.IsBalanceOK(balanceLimit) {
		return
	}

	mes := "🚨 Баланс меньше " + strconv.FormatFloat(balanceLimit, 'f', 2, 64) + " ₽!\nНадо пополнять."
	b.Send(sender, mes)
	SendInvoiceMenu(b, sender, menu)
}

func SendInvoiceMenu(b *tb.Bot, sender *tb.User, menu *tb.ReplyMarkup) {
	b.Send(sender, "Какой нужен счёт?", menu)
}

func Invoice(user *delimobil.User, b *tb.Bot, sender *tb.User, amount ...float64) {
	if err := user.Auth(user.Login, user.Password); err != nil {
		log.Print(err)
		b.Send(sender, err.Error())
		return
	}
	var (
		invoice *delimobil.File
		err     error
	)
	if amount != nil {
		invoice, err = user.CreateInvoice(amount[0])
	} else {
		invoice, err = user.LastInvoice()
	}
	if err != nil {
		log.Print(err)
		b.Send(sender, err.Error())
		return
	}
	doc := &tb.Document{File: tb.FromReader(invoice.Data)}
	doc.FileName = invoice.FileName
	doc.MIME = invoice.MIME
	b.Send(sender, doc)
}

func UserCredentials(client *firestore.Client, ctx *context.Context, ID int) (user *delimobil.User, err error) {
	userDocRef := client.Collection("Users").Doc(strconv.Itoa(ID))
	authError := errors.New("🤔 Кажется, мы ещё не знакомы. Отправь команду\n/auth login password")

	userDoc, err := userDocRef.Get(*ctx)
	if err != nil {
		log.Print("can't get anything from firestore", err)
		return nil, authError
	}

	userGob, err := userDoc.DataAt("gob")
	if err != nil {
		return nil, authError
	}

	user = new(delimobil.User)
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

	if user.IsValid() {
		return user, nil
	}

	return SetUserCredentials(client, ctx, ID, user.Login, user.Password)
}

func SetUserCredentials(client *firestore.Client, ctx *context.Context, ID int, login, password string) (user *delimobil.User, err error) {
	user = new(delimobil.User)
	userDocRef := client.Collection("Users").Doc(strconv.Itoa(ID))

	if err := user.Auth(login, password); err != nil {
		return nil, err
	}
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(user); err != nil {
		return nil, err
	}

	if _, err := userDocRef.Set(*ctx, map[string]interface{}{"gob": base64.StdEncoding.EncodeToString(buf.Bytes())}); err != nil {
		return nil, err
	}

	return user, nil
}
