package main

import (
	"strconv"

	tele "gopkg.in/tucnak/telebot.v3"
)

func NotifyAboutBalanceChange(b *tele.Bot) {
	log.Trace("Notifying users about changes...")
	companyLastBalance := make(map[int]*Company)

	companyDocRefs, err := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("companies").DocumentRefs(ctx).GetAll()
	if err != nil {
		log.Warn(err)
	}

	for _, companyDocRef := range companyDocRefs {
		companyId, err := strconv.Atoi(companyDocRef.ID)
		if err != nil {
			log.Warn(err)
		}

		companyLogger := log.WithField("companyId", companyId)
		company, err := LoadCompany(companyId)
		if err != nil {
			companyLogger.Warn(err)
		}

		if err := company.SetInfo(); err != nil {
			companyLogger.Warn(err)
		}
		companyLastBalance[company.Id] = company
		companyLogger.Trace("Saved updated company to the map")
	}
	log.Trace("Checked all companies")

	userDocRefs, err := client.Collection(appConfig.ProjectID).Doc(appConfig.Environment).Collection("users").DocumentRefs(ctx).GetAll()
	if err != nil {
		log.Warn(err)
	}
	for _, userDocRef := range userDocRefs {
		userId, err := strconv.ParseInt(userDocRef.ID, 10, 64)
		if err != nil {
			log.Warn(err)
		}

		userLogger := log.WithField("userId", userId)
		user, err := LoadUser(userId)
		if err != nil {
			userLogger.Warn(err)
		}

		if company, ok := companyLastBalance[user.CompanyId]; ok {
			companyLogger := userLogger.WithField("companyId", company.Id)
			companyLogger.Trace("Found user's company in balances list")

			if company.Permissions(user) == "" {
				companyLogger.Warn("User doesn't have permissions for this company")
				break
			}

			balance := company.Balance
			if balance != user.LastBalance {
				companyLogger.Trace("User's last balance defers")
				mes := "💸 Баланс компании " + company.Info.Name + " изменился\n" +
					"Текущий баланс: " + strconv.FormatFloat(company.Info.Balance, 'f', 2, 64) + " ₽"
				if _, err := b.Send(user, mes); err == nil {
					companyLogger.Trace("Notifyed user! Updating user's last balance...")
					user.LastBalance = balance
					if err := user.SaveUser(); err == nil {
						companyLogger.Trace("Updated user's last balance!")
					} else {
						companyLogger.Warn(err)
					}
				} else {
					companyLogger.Warn(err)
				}
			}
			companyLogger.Trace("User's last balance is actual, not notifying")
		}
	}
	log.Trace("Checked all users")
}
