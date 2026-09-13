## sms
Send SMS text messages from the command line.

### Configuration
Needs a KEY from the https://textbelt.com service: `svckey = KEY`

### Usage

```text
sms v1.4.0
Send an SMS message through textbelt.com
github.com/queone/gkit/tree/main/cmd/sms

Usage
  sms NUMBER MESSAGE  Send MESSAGE to NUMBER with the key in ~/.config/sms/config.ini
  sms -y              Create a skeleton ~/.config/sms/config.ini

  Visit https://textbelt.com for more info.

Options
  -y              Write the skeleton config file and exit
  -v, --version   Print sms v1.4.0 and exit
  -h, -?, --help  Show this help and exit
```
Run with the 2 obvious arguments (cellphone number & the actual message):

```bash
$ sms
SMS CLI utility 1.3.0
sms <CellPhoneNum> <Message>
sms -y Create skeleton ~/.config/sms/config.ini file
Visit https://textbelt.com for more info.

sms 2015554444 "Hello world"
```
