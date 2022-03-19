package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/Bnei-Baruch/wf-srv/workflow"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"net"
	"os/exec"
	"strings"
	"time"
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

func (a *App) ConMQTT() error {
	var err error

	a.Msg.Conn = connect()
	var sessionExpiryInterval = uint32(math.MaxUint32)

	cp := &paho.Connect{
		ClientID:     common.EP + "-exec_mqtt_client",
		KeepAlive:    10,
		CleanStart:   true,
		Username:     common.USERNAME,
		Password:     []byte(common.PASSWORD),
		UsernameFlag: true,
		PasswordFlag: true,
		Properties: &paho.ConnectProperties{
			SessionExpiryInterval: &sessionExpiryInterval,
		},
	}

	a.Msg.SetErrorLogger(NewPahoLogAdapter(zerolog.DebugLevel))
	debugLog := NewPahoLogAdapter(zerolog.DebugLevel)
	a.Msg.SetDebugLogger(debugLog)
	a.Msg.PingHandler.SetDebug(debugLog)
	a.Msg.Router.SetDebugLogger(debugLog)

	ca, err := a.Msg.Connect(context.Background(), cp)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("client.Connect")
	}
	if ca.ReasonCode != 0 {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT connect error: %d - %s", ca.ReasonCode, ca.Properties.ReasonString)
	}

	sa, err := a.Msg.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			common.WorkflowExec: {QoS: byte(1)},
		},
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("client.Subscribe")
	}
	if sa.Reasons[0] != byte(1) {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT subscribe error: %d ", sa.Reasons[0])
	}

	a.Msg.Router.RegisterHandler(common.WorkflowExec, a.execMessage)

	return nil
}

func connect() net.Conn {
	var conn net.Conn
	var err error

	for {
		conn, err = tls.Dial("tcp", common.SERVER, nil)
		if err != nil {
			log.Error().Str("source", "MQTT").Err(err).Msg("conn.Dial")
			time.Sleep(1 * time.Second)
		} else {
			break
		}
	}

	return conn
}

func (a *App) LostMQTT(err error) {
	log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
	time.Sleep(1 * time.Second)
	if err := a.Msg.Disconnect(&paho.Disconnect{ReasonCode: 0}); err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Reconnecting..")
	}
	time.Sleep(1 * time.Second)
	a.initMQTT()
}

func (a *App) execMessage(m *paho.Publish) {
	//log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", string(m.Payload), m.Topic)
	id := "false"
	s := strings.Split(m.Topic, "/")
	p := string(m.Payload)

	if s[0] == common.ExtPrefix && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	log.Debug().Str("source", "MQTT").Msgf("Topic parser: %s\n", id)

	if id != "false" {
		cmd := exec.Command("/opt/wfexec/"+id+".sh", p, common.EP)
		cmd.Dir = "/opt/wfexec/"
		_, err := cmd.CombinedOutput()

		if id == "sync" || id == "storage" || common.EP == "wf-srv" {
			notifyMessage(m.Payload)
		}

		if err != nil {
			log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
		}
	}

	//s.Out = string(out)
	//json.Unmarshal(out, &s.Result)
}

func notifyMessage(m []byte) {
	log.Debug().Str("source", "MQTT").Msgf("prepare notify mail..\n")
	var file workflow.Files

	// Unquote
	uq := strings.ReplaceAll(string(m), "\n", "")
	uq = strings.ReplaceAll(uq, "\\", "")
	m = []byte(uq)

	//log.Debug().Str("source", "MAIL").Msgf("Unquote: %s \n", m)

	err := json.Unmarshal(m, &file)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Unmarshal error")
		return
	}

	log.Debug().Str("source", "MQTT").Msgf("Check File Name: %s , with ID: %s \n", file.FileName, file.ProductID)

	err, exist := IsExist(file.FileName)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Fail to check file name")
		return
	}
	if exist == true {
		return
	}

	SendEmail(file.FileName, file.ProductID)
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

	pa, err := a.Msg.Publish(context.Background(), &paho.Publish{
		QoS:     byte(1),
		Retain:  false,
		Topic:   topic,
		Payload: message,
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT Publish error: %d ", pa.Properties.ReasonString)
	}
}

func (a *App) SendMessage(source string, message []byte) {
	//message, err := json.Marshal(m)
	//if err != nil {
	//	log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	//}
	var topic string

	switch source {
	case "upload":
		topic = common.MonitorUploadTopic
	case "convert":
		topic = common.MonitorConvertTopic
	case "storage":
		topic = "exec/workflow/storage/sync"
	}

	pa, err := a.Msg.Publish(context.Background(), &paho.Publish{
		QoS:     byte(1),
		Retain:  false,
		Topic:   topic,
		Payload: message,
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Publish: Topic - " + topic + " " + pa.Properties.ReasonString)
	}

	log.Debug().Str("source", "MQTT").Str("json", string(message)).Msg("Publish: Topic - " + topic)
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
