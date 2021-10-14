package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"strconv"

	deli "github.com/fuksman/delimobil"
)

type Company struct {
	*deli.Company
}

func NewCompany(login, password string) (company *Company) {
	deliCompany := deli.NewCompany(login, password)
	return &Company{deliCompany}
}

func (company *Company) SaveCompany() error {
	companyLogger := log.WithField("companyId", company.Id)
	companyLogger.Trace("Saving company...")
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(company); err != nil {
		companyLogger.Warn(err)
		return err
	}

	companyDocRef := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("companies").Doc(strconv.Itoa(company.Id))
	if _, err := companyDocRef.Set(ctx, map[string]interface{}{"gob": base64.StdEncoding.EncodeToString(buf.Bytes())}); err != nil {
		companyLogger.Warn(err)
		return err
	}

	companyLogger.Info("Saved!")
	return nil
}

func LoadCompany(id int) (company *Company, err error) {
	companyLogger := log.WithField("companyId", id)
	companyLogger.Trace("Loading company data...")
	companyDocRef := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("companies").Doc(strconv.Itoa(id))

	companyDoc, err := companyDocRef.Get(ctx)
	if err != nil {
		companyLogger.Warn(err)
		return nil, err
	}

	userGob, err := companyDoc.DataAt("gob")
	if err != nil {
		companyLogger.Warn(err)
		return nil, err
	}

	by, err := base64.StdEncoding.DecodeString(userGob.(string))
	if err != nil {
		companyLogger.Warn(err)
		return nil, err
	}
	buf := bytes.Buffer{}
	buf.Write(by)
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&company)
	if err != nil {
		companyLogger.Warn(err)
		return nil, err
	}

	companyLogger.Info("Loaded!")

	companyLogger.Trace("Updating token...")

	if err := company.Authenticate(); err != nil {
		companyLogger.Warn(err)
		return nil, err
	}
	companyLogger.Info("Token updated.")

	return company, nil
}

func RemoveCompany(id int) error {
	companyLogger := log.WithField("companyId", id)
	companyLogger.Trace("Removing company data...")
	companyDocRef := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("companies").Doc(strconv.Itoa(id))
	if _, err := companyDocRef.Delete(ctx); err != nil {
		companyLogger.Warn(err)
		return err
	}
	companyLogger.Info("Deleted!")
	return nil
}
