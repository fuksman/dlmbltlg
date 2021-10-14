package main

import (
	"errors"

	deli "github.com/fuksman/delimobil"
	tele "gopkg.in/tucnak/telebot.v3"
)

func Invoice(tlg *tele.Context, amount ...float64) error {
	userLogger := log.WithField("userId", (*tlg).Sender().ID)

	company, ok := (*tlg).Get("company").(*Company)
	if !ok {
		err := errors.New("ошибка получения информации о компании")
		log.Warn(err)
		return (*tlg).Send(err.Error())
	}

	var (
		invoice *deli.File
		err     error
	)
	if amount != nil {
		userLogger.Trace("Creating new invoice...")
		invoice, err = company.CreateInvoice(amount[0])
	} else {
		userLogger.Trace("Retrieving last invoice...")
		invoice, err = company.LastFileByType("invoice")
	}
	if err != nil {
		userLogger.Warn("Can't create/retrieve invoice.")
		(*tlg).Send(err.Error())
		return err
	}
	userLogger.Info("Created/recieved invoice.")
	doc := &tele.Document{File: tele.FromReader(invoice.Data)}
	doc.FileName = invoice.FileName
	doc.MIME = invoice.MIME
	return (*tlg).Send(doc, adminMenu)
}
