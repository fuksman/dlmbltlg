# dlmbltlg
Telegram bot to work with Delimobil b2b-account.

This application provides abilities to:
* Authenticate in a bot with Delimobil admin's credentials or as the employee of existing company
* Get balance with automatic notifications about its changes
* Get last rides list
* Get last documents from Delimobil
* Generate new invoices

## Running

### Source code

Run:
```zsh
DLMBLTLG="{path_to_app_config}" GOOGLE_APPLICATION_CREDENTIALS="{path_to_Google_ADC}" go run dlmbltlg
```

More about `{{path_to_app_config}}` in ["App configuration file"](#app-configuration-file)

More about `{path_to_Google_ADC}`: https://cloud.google.com/docs/authentication/production

### Command-line
Download [last version](https://github.com/fuksman/dlmbltlg/releases/latest) from releases.

The executable relies on several env variables:
* `DLMBLTLG="{path_to_app_config}"` (more about [configuration file](#app-configuration-file))
* `GOOGLE_APPLICATION_CREDENTIALS={path_to_Google_ADC}` (used for Firestore authentification, read more: https://cloud.google.com/docs/authentication/production)

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
There is an action which builds an executables for `linux/396` and `darwin/arm64` which triggers on:
```zsh
git tag {vN.N.N}
git push --tag
```


## App configuration file
App configuration should be a JSON-file following structure (all fields are required):
```(json)
{
  "environment": {"test" or "prod"},
  "telegram_token": {token},
  "project_id": {Google Cloud Project ID},
  "check_delay": {number of seconds between balance change checking}
}
```

## Related projects

Here's a list of other related projects:

- [Delimobil b2b API Wrapper for golang](https://github.com/fuksman/delimobil)