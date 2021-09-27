package delimobil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

var apihost = "https://b2b-api.delitime.ru"
var b2bhandler = "/b2b/company/"

type User struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	JWT      Auth
	Org      Company
}

type Auth struct {
	Token   string
	Company []struct {
		ID        float64 `json:"company_id"`
		FirstName string  `json:"first_name"`
		LastName  string  `json:"last_name"`
	} `json:"user"`
	jwt.StandardClaims
}

type Company struct {
	Id               int
	Name             string
	Balance          float64
	CanCreateInvoice bool
	MinInvoiceAmount float64
	Rides            Rides
}

type Ride struct {
	RentID            int       `json:"rent_id"`
	RentStartTime     time.Time `json:"rent_start_time"`
	RentEndTime       time.Time `json:"rent_end_time"`
	Duration          int       `json:"duration"`
	Cost              float64   `json:"cost"`
	Currency          string    `json:"currency"`
	Car               string    `json:"car"`
	VehicleNumber     string    `json:"vehicle_number"`
	ClientBio         string    `json:"client_bio"`
	Distance          int       `json:"distance"`
	StartPointAddress string    `json:"start_point_address"`
	EndPointAddress   string    `json:"end_point_address"`
}

type Rides []Ride

type File struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	PeriodStart string `json:"period_start"`
	PeriodEnd   string `json:"period_end"`
	Status      string `json:"status"`
	URL         string `json:"url"`
	Data        io.Reader
	FileName    string
	MIME        string
}

func (user *User) IsValid() bool {
	return (user.JWT.StandardClaims.Valid() == nil) && user.JWT.ExpiresAt != 0
}

func (user *User) Auth(login, password string) error {
	if user.IsValid() {
		return nil
	}

	user.setCredentials(login, password)

	endpoint := apihost + "/b2b/auth"
	userData := map[string]string{
		"login":    user.Login,
		"password": user.Password,
	}
	jsonUser, err := json.Marshal(userData)
	if err != nil {
		log.Print(err)
		return err
	}

	resp, err := http.Post(endpoint, "application/json", bytes.NewBuffer(jsonUser))
	if err != nil {
		log.Print(err)
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return err
	}

	if resp.StatusCode > 299 {
		err := errors.New("bad request, status code: " + strconv.Itoa(resp.StatusCode))
		log.Print(err)
		return err
	}

	var temp struct {
		Token   string `json:"message"`
		Success bool   `json:"success"`
	}

	if json.Unmarshal(body, &temp); !temp.Success {
		err := errors.New("can't retrieve information via API")
		log.Print(err)
		return err
	}

	token, _, err := new(jwt.Parser).ParseUnverified(temp.Token, &Auth{})
	if err != nil {
		log.Print(err)
		return err
	}

	if claims, ok := token.Claims.(*Auth); ok {
		claims.Token = temp.Token
		user.JWT = *claims
		return nil
	} else {
		err := errors.New("JWT is not valid")
		log.Print(err)
		return err
	}
}

func (user *User) setCredentials(login, password string) {
	user.Login = login
	user.Password = password
}

func (user *User) CompanyId() string {
	return strconv.FormatFloat(user.JWT.Company[0].ID, 'f', 0, 64)
}

func (user *User) SetCompanyInfo() error {
	endpoint := apihost + b2bhandler + user.CompanyId() + "/info"
	body, err := MakeAPIRequest("GET", endpoint, nil, &user.JWT)
	if err != nil {
		log.Print(err)
		return err
	}

	var temp struct {
		CompanyInfo struct {
			Id               int     `json:"id"`
			Name             string  `json:"company_name"`
			Balance          float64 `json:"total_sum"`
			CanCreateInvoice bool    `json:"isCreatingInvoicesAllowed"`
			MinInvoiceAmount float64 `json:"minInvoiceAmount"`
			Rides            Rides
		} `json:"message"`
		Success bool `json:"success"`
	}
	json.Unmarshal(body, &temp)

	if temp.Success {
		user.Org = Company(temp.CompanyInfo)
		return nil
	} else {
		err := errors.New("can't retrieve information via API")
		log.Print(err)
		return err
	}
}

func (user *User) SetRidesInfo(limit, page int) error {
	auth := &user.JWT
	company := &user.Org
	endpoint := apihost + b2bhandler + user.CompanyId() + "/transfers/all?limit=" + strconv.Itoa(limit) + "&page=" + strconv.Itoa(page)
	body, err := MakeAPIRequest("GET", endpoint, nil, auth)
	if err != nil {
		log.Print(err)
		return err
	}

	var temp struct {
		Rides   Rides `json:"message"`
		Success bool  `json:"success"`
	}
	json.Unmarshal(body, &temp)

	if temp.Success {
		company.Rides = temp.Rides
		return nil
	} else {
		err := errors.New("can't retrieve information via API")
		log.Print(err)
		return err
	}
}

func (ride *Ride) String() string {
	if ride.Currency == "rub" {
		ride.Currency = "₽"
	}
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		loc = time.FixedZone("UTC", 0)
	}
	return ride.RentStartTime.In(loc).Format("02.01.06 15:04") + "—" + ride.RentEndTime.In(loc).Format("15:04") +
		" " + strconv.Itoa(ride.Duration) + " мин., " + strconv.FormatFloat(ride.Cost, 'f', 2, 64) + " " + ride.Currency + "\n" +
		ride.StartPointAddress + " 👉 " + ride.EndPointAddress + ", " + ride.ClientBio + ", " + ride.Car + " (" + strings.ToUpper(ride.VehicleNumber) + ")"
}

func (rides Rides) String() (list string) {
	list = ""
	for _, ride := range rides {
		list += ride.String() + "\n\n"
	}
	return
}

func (user *User) IsBalanceOK(limit float64) bool {
	return user.Org.Balance > limit
}

func (user *User) CreateInvoice(amount float64) (invoice *File, err error) {
	auth := &user.JWT
	company := &user.Org
	if !company.CanCreateInvoice {
		err := errors.New("user is not allowed to create invoices")
		log.Print(err)
		return nil, err
	}
	if amount < company.MinInvoiceAmount {
		err := errors.New("can't crate invoice with this amount, minimim amount is " +
			strconv.FormatFloat(company.MinInvoiceAmount, 'f', 2, 64))
		log.Print(err)
		return nil, err
	}
	var invoiceData struct {
		Invoice struct {
			Amount      float64 `json:"amount"`
			Bill_number int     `json:"bill_number"`
			Description string  `json:"description"`
			Created_at  string  `json:"created_at"`
		} `json:"invoice"`
	}

	invoiceData.Invoice.Amount = amount
	invoiceData.Invoice.Bill_number = company.Id
	invoiceData.Invoice.Description = "Счет"
	invoiceData.Invoice.Created_at = time.Now().Format("2006-01-02T15:04:05.000Z")

	jsonInvoice, err := json.Marshal(invoiceData)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	endpoint := apihost + b2bhandler + user.CompanyId() + "/invoice/new"
	body, err := MakeAPIRequest("POST", endpoint, bytes.NewBuffer(jsonInvoice), auth)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	var temp struct {
		Message float64 `json:"message"`
		Success bool    `json:"success"`
	}
	json.Unmarshal(body, &temp)

	if temp.Success {
		return user.LastInvoice()
	} else {
		err := errors.New("can't retrieve information via API")
		log.Print(err)
		return nil, err
	}
}

func (user *User) LastInvoice() (invoice *File, err error) {
	auth := &user.JWT
	endpoint := apihost + b2bhandler + user.CompanyId() + "/docs/" + strconv.Itoa(time.Now().Year()) + "/" + strconv.Itoa((int)(time.Now().Month()))
	body, err := MakeAPIRequest("GET", endpoint, nil, auth)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	var temp struct {
		Files   []File `json:"message"`
		Success bool   `json:"success"`
	}
	json.Unmarshal(body, &temp)

	if !temp.Success {
		log.Print(err)
		return nil, err
	}

	invoice = &temp.Files[0]
	for _, file := range temp.Files {
		if file.URL > invoice.URL {
			invoice = &file
		}
	}
	body, err = MakeAPIRequest("GET", apihost+invoice.URL, nil, auth)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	invoice.Data = bytes.NewReader(body)
	invoice.FileName = invoice.Title + ".pdf"
	invoice.MIME = "application/pdf"
	return invoice, nil
}

func MakeAPIRequest(method string, endpoint string, reqbody io.Reader, auth *Auth) (body []byte, err error) {
	client := &http.Client{}

	req, err := http.NewRequest(method, endpoint, reqbody)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+auth.Token)
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Print(err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Print(err)
		return nil, err
	}

	if resp.StatusCode > 299 {
		err := errors.New("bad request, status code: " + strconv.Itoa(resp.StatusCode))
		log.Print(err)
		return nil, err
	}

	return body, nil
}

func (user *User) Name() string {
	return user.JWT.Company[0].FirstName + " " + user.JWT.Company[0].LastName
}
