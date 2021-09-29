# dlmbltlg
Telegram bot to work with Delimobil b2b-account.

This application provides abilities to:
* Authenticate in a bot with Delimobil user's credentials
* Get balance
* Get last rides list
* Get last invoice generated in the current month
* Generate and get new invoice

## Running

### Source code

Run:
```zsh
GOOGLE_APPLICATION_CREDENTIALS="{path_to_Google_ADC}" DLMBLENV="{env}" go run main.go
```

More about `{path_to_Google_ADC}`: https://cloud.google.com/docs/authentication/production

Options for `{env}`: `prod` or `test`

### Command-line
Download [last version](https://github.com/fuksman/dlmbltlg/releases/latest) from releases.

The executable relies on several env variables:
* `GOOGLE_APPLICATION_CREDENTIALS={path_to_Google_ADC}` (used for Firestore authentification, read more: https://cloud.google.com/docs/authentication/production)
* `DLMBLENV={env}`, where `{env}` is `prod` or `test`

Run:
```zsh
./dlmbltlg
```


## Building

### Localy
Run:
```zsh
go build
```

### Github Actions
There is an action which builds an executables for `linux/396` and `darwin/amd64` which triggers on:
```zsh
git tag {vN.N.N}
git push --tag
```


## Firestore configuration
Database collections:
```
Users
- {Telegram Chat ID}
TestUsers
- {Telegram Chat ID}
Secrets
- Telegram
-- prod: {Telegram Bot Token}
-- test: {Telegram Bot Token}
```

## Related projects

Here's a list of other related projects:

- [Delimobil b2b API Wrapper for golang](https://github.com/fuksman/delimobil)