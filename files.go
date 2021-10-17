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
		return (*tlg).Send(err.Error())
	}
	userLogger.Info("Created/recieved invoice.")
	doc := &tele.Document{File: tele.FromReader(invoice.Data)}
	doc.FileName = invoice.FileName
	doc.MIME = invoice.MIME
	return (*tlg).Send(doc, menu)
}

func LastClosingDocuments(tlg *tele.Context) error {
	user, company, menu, err := ReadContext(*tlg)
	if err != nil {
		return (*tlg).Send(err.Error(), startMenu)
	}

	var (
		upd         *deli.File
		rentsDetail *deli.File
		userLogger  = log.WithField("userId", user.Id)
	)

	userLogger.Trace("Retrieving last upd...")
	upd, err = company.LastFileByType("upd")
	if err != nil {
		userLogger.Warn("Can't retrieve last upd.")
		return (*tlg).Send(err.Error())
	}
	userLogger.Info("Recieved upd.")
	doc := &tele.Document{File: tele.FromReader(upd.Data)}
	doc.FileName = upd.FileName
	doc.MIME = upd.MIME
	(*tlg).Send(doc, menu)

	userLogger.Trace("Retrieving last rentsDetail...")
	rentsDetail, err = company.LastFileByType("rentsDetail")
	if err != nil {
		userLogger.Warn("Can't retrieve last rentsDetail.")
		return (*tlg).Send(err.Error())
	}
	userLogger.Info("Recieved rentsDetail.")
	doc = &tele.Document{File: tele.FromReader(rentsDetail.Data)}
	doc.FileName = rentsDetail.FileName
	doc.MIME = rentsDetail.MIME
	return (*tlg).Send(doc, menu)
}
