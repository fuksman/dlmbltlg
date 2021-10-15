package main

import (
	deli "github.com/fuksman/delimobil"
	tele "gopkg.in/tucnak/telebot.v3"
)

func Invoice(tlg *tele.Context, amount ...float64) error {
	user, company, menu, err := ReadContext(*tlg)
	if err != nil {
		return (*tlg).Send(err.Error(), startMenu)
	}

	var (
		invoice    *deli.File
		userLogger = log.WithField("userId", user.Id)
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
	return (*tlg).Send(doc, menu)
}