package main

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"strconv"
)

type User struct {
	Id        int64
	Phone     string
	CompanyId int
	Admin     bool
}

func (user *User) SaveUser() error {
	userLogger := log.WithField("userId", user.Id)
	userLogger.Trace("Saving user...")
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(user); err != nil {
		userLogger.Warn(err)
		return err
	}

	userDocRef := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("users").Doc(strconv.FormatInt(user.Id, 10))
	if _, err := userDocRef.Set(ctx, map[string]interface{}{"gob": base64.StdEncoding.EncodeToString(buf.Bytes())}); err != nil {
		userLogger.Warn(err)
		return err
	}

	userLogger.Info("Saved!")
	return nil
}

func LoadUser(id int64) (user *User, err error) {
	userLogger := log.WithField("userId", id)
	userLogger.Trace("Loading user data...")
	userDocRef := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("users").Doc(strconv.FormatInt(id, 10))

	userDoc, err := userDocRef.Get(ctx)
	if err != nil {
		userLogger.Warn(err)
		return nil, err
	}

	userGob, err := userDoc.DataAt("gob")
	if err != nil {
		userLogger.Warn(err)
		return nil, err
	}

	by, err := base64.StdEncoding.DecodeString(userGob.(string))
	if err != nil {
		userLogger.Warn(err)
		return nil, err
	}
	buf := bytes.Buffer{}
	buf.Write(by)
	dec := gob.NewDecoder(&buf)
	err = dec.Decode(&user)
	if err != nil {
		userLogger.Warn(err)
		return nil, err
	}

	userLogger.Info("Loaded!")
	return user, nil
}

func RemoveUser(id int64) error {
	userLogger := log.WithField("userId", id)
	userLogger.Trace("Removing user data...")
	userDocRef := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("users").Doc(strconv.FormatInt(id, 10))
	if _, err := userDocRef.Delete(ctx); err != nil {
		userLogger.Warn(err)
		return err
	}
	userLogger.Info("Deleted!")
	return nil
}

func (user *User) FindCompany() (company *Company, err error) {
	userLogger := log.WithField("userId", user.Id)
	userLogger.Trace("Looking for a company by user data...")
	companyDocRefs, err := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("companies").DocumentRefs(ctx).GetAll()
	if err != nil {
		userLogger.Warn(err)
		return nil, err
	}
	for _, companyDocRef := range companyDocRefs {
		companyId, err := strconv.Atoi(companyDocRef.ID)
		if err != nil {
			userLogger.Warn(err)
			return nil, err
		}
		company, err = LoadCompany(companyId)
		if err != nil {
			userLogger.Warn(err)
			return nil, err
		}
		if err = company.SetEmployees(); err != nil {
			userLogger.Warn(err)
			return nil, err
		}
		exist, err := company.HasEmployee(user.Phone)
		if err != nil {
			userLogger.Warn(err)
			return nil, err
		}
		if exist {
			user.CompanyId = company.Id
			if err = user.SaveUser(); err != nil {
				userLogger.Warn(err)
				return nil, err
			}
			userLogger.Trace("Found user in a company!")
			return company, nil
		}
	}

	userLogger.Trace("Didn't find user in any company")
	return nil, nil
}

func (user *User) IsActive() bool {
	userLogger := log.WithField("userId", user.Id)
	userLogger.Trace("Checking if user is still employee...")
	company, err := LoadCompany(user.CompanyId)
	if err != nil {
		userLogger.Warn(err)
		return false
	}

	active, err := company.HasEmployee(user.Phone)
	if err != nil {
		userLogger.Warn(err)
		return false
	}
	userLogger.Trace("User activity in the company: " + strconv.FormatBool(active))
	return active
}
