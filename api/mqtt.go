package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/Bnei-Baruch/wf-srv/workflow"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/smtp"
	"os/exec"
	"strings"
)

type MqttPayload struct {
	Action  string      `json:"action,omitempty"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	Source  string      `json:"src,omitempty"`
	Error   error       `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Result  string      `json:"result,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (a *App) SubMQTT(c mqtt.Client) {
	log.Info().Str("source", "MQTT").Msg("- Connected -")

	//FIXME: OLd exec flow - will be deprecated
	if token := a.Msg.Subscribe(common.WorkflowExec, byte(2), a.execMessage); token.Wait() && token.Error() != nil {
		log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	} else {
		log.Info().Str("source", "MQTT").Msg("Subscription - " + common.WorkflowExec)
	}

	if token := a.Msg.Subscribe(common.ExtPrefix+common.WorkflowExec, byte(2), a.execMessage); token.Wait() && token.Error() != nil {
		log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	} else {
		log.Info().Str("source", "MQTT").Msg("Subscription - " + common.ExtPrefix + common.WorkflowExec)
	}

	//TODO: Will come to chenge old exec flow
	//if token := a.Msg.Subscribe(common.WorkflowTopic, byte(2), wf.MqttMessage); token.Wait() && token.Error() != nil {
	//	log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	//} else {
	//	log.Info().Str("source", "MQTT").Msg("Subscription - " + common.WorkflowTopic)
	//}
	//
	//if token := a.Msg.Subscribe(common.ExtPrefix+common.WorkflowTopic, byte(2), wf.MqttMessage); token.Wait() && token.Error() != nil {
	//	log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	//} else {
	//	log.Info().Str("source", "MQTT").Msg("Subscription - " + common.ExtPrefix + common.WorkflowTopic)
	//}
}

func (a *App) LostMQTT(c mqtt.Client, err error) {
	log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
}

func (a *App) execMessage(c mqtt.Client, m mqtt.Message) {
	log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	id := "false"
	s := strings.Split(m.Topic(), "/")
	p := string(m.Payload())

	if s[0] == common.ExtPrefix && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	if id != "false" {
		cmd := exec.Command("/opt/wfexec/"+id+".sh", p, common.EP)
		cmd.Dir = "/opt/wfexec/"
		_, err := cmd.CombinedOutput()

		if id == "sync" || id == "storage" || common.EP == "wf-nas" {
			sendEmail(m.Payload())
		}

		if err != nil {
			log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
		}
	}

	//s.Out = string(out)
	//json.Unmarshal(out, &s.Result)
}

func sendEmail(m []byte) {
	log.Debug().Str("source", "MAIL").Msgf("Sending mail..\n")
	var file workflow.Files

	// Unquote
	uq := strings.ReplaceAll(string(m), "\n", "")
	uq = strings.ReplaceAll(uq, "\\", "")
	m = []byte(uq)

	log.Debug().Str("source", "MAIL").Msgf("Unquote: %s \n", m)

	err := json.Unmarshal(m, &file)
	if err != nil {
		log.Error().Str("source", "MAIL").Err(err).Msg("Unmarshal error")
		return
	}

	err, exist := IsExist(file.FileName)
	if err != nil {
		log.Error().Str("source", "MAIL").Err(err).Msg("Fail to check file name")
		return
	}
	if exist == true {
		return
	}

	log.Debug().Str("source", "MAIL").Msgf("File Name: %s \n", file.FileName)

	o := strings.Split(file.FileName, "_")[1]
	if o == "o" {
		l := strings.Split(file.FileName, "_")[0]
		var to []string
		if l == "heb" {
			to = []string{"amnonbb@gmail.com", "alex.mizrachi@gmail.com", "oren.yair@gmail.com", "dani3l.rav@gmail.com"}
		} else if l == "rus" {
			to = []string{"amnonbb@gmail.com", "alex.mizrachi@gmail.com", "dmitrysamsonnikov@gmail.com"}
		} else {
			to = []string{"amnonbb@gmail.com", "alex.mizrachi@gmail.com"}
		}
		user := common.MAIL_USER
		password := common.MAIL_PASS
		smtpHost := common.MAIL_HOST
		from := common.MAIL_FROM
		smtpPort := "587"

		msg := []byte("From: " + from + "\r\n" +
			"Subject: " + file.FileName + "\r\n" +
			"\r\n" +
			"Product ID: " + file.ProductID + "\r\n")
		auth := smtp.PlainAuth("", user, password, smtpHost)
		err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, msg)
		if err != nil {
			log.Error().Str("source", "MAIL").Err(err).Msg("SendMail error")
		} else {
			log.Debug().Str("source", "MAIL").Msg("Mail sent.\n")
		}
	}
}

func (a *App) SendRespond(id string, m *MqttPayload) {
	var topic string

	if id == "false" {
		topic = common.ServiceDataTopic + common.EP
	} else {
		topic = common.ServiceDataTopic + common.EP + "/" + id
	}
	message, err := json.Marshal(m)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	}

	if token := a.Msg.Publish(topic, byte(2), false, message); token.Wait() && token.Error() != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Send Respond")
	}
}

func (a *App) SendMessage(source string, message []byte) {
	var topic string
	//var m interface{}
	//date := time.Now().Format("2006-01-02")

	//message, err := json.Marshal(m)
	//if err != nil {
	//	log.Error().Str("monitor", "MQTT").Err(err).Msg("Message parsing")
	//}

	switch source {
	case "upload":
		topic = common.MonitorUploadTopic
	case "convert":
		topic = common.MonitorConvertTopic
	case "storage":
		topic = "exec/workflow/storage/sync"
	}

	if token := a.Msg.Publish(topic, byte(0), true, message); token.Wait() && token.Error() != nil {
		log.Error().Str("monitor", "MQTT").Err(token.Error()).Msg("Report Monitor")
	}
}

func (a *App) InitLogMQTT() {
	mqtt.DEBUG = NewPahoLogAdapter(zerolog.InfoLevel)
	mqtt.WARN = NewPahoLogAdapter(zerolog.WarnLevel)
	mqtt.CRITICAL = NewPahoLogAdapter(zerolog.ErrorLevel)
	mqtt.ERROR = NewPahoLogAdapter(zerolog.ErrorLevel)
}

type PahoLogAdapter struct {
	level zerolog.Level
}

func NewPahoLogAdapter(level zerolog.Level) *PahoLogAdapter {
	return &PahoLogAdapter{level: level}
}

func (a *PahoLogAdapter) Println(v ...interface{}) {
	log.Debug().Str("source", "MQTT").Msgf("%s", fmt.Sprint(v...))
}

func (a *PahoLogAdapter) Printf(format string, v ...interface{}) {
	log.Debug().Str("source", "MQTT").Msgf("%s", fmt.Sprintf(format, v...))
}
