package api

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/Bnei-Baruch/wf-srv/wf"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"math"
	"net"
	"os/exec"
	"strings"
	"time"
)

type Mqtt struct {
	mqtt *paho.Client
	WF   wf.WF
}

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

type MQ interface {
	SendMessage(string, []byte)
	Init() error
	SendRespond(string, *MqttPayload)
}

func NewMqtt(mqtt *paho.Client) MQ {
	return &Mqtt{
		mqtt: mqtt,
	}
}

func (m *Mqtt) Init() error {
	m.mqtt = paho.NewClient(paho.ClientConfig{
		ClientID:      common.EP + "-exec_mqtt_client",
		OnClientError: m.lostMQTT,
	})

	m.WF = wf.NewWorkFlow(m.mqtt)

	if err := m.conMQTT(); err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("initialize mqtt connection")
	}

	return nil
}

func (m *Mqtt) conMQTT() error {
	var err error

	m.mqtt.Conn = connect()
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

	m.mqtt.SetErrorLogger(NewPahoLogAdapter(zerolog.DebugLevel))
	debugLog := NewPahoLogAdapter(zerolog.DebugLevel)
	m.mqtt.SetDebugLogger(debugLog)
	m.mqtt.PingHandler.SetDebug(debugLog)
	m.mqtt.Router.SetDebugLogger(debugLog)

	ca, err := m.mqtt.Connect(context.Background(), cp)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("client.Connect")
	}
	if ca.ReasonCode != 0 {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT connect error: %d - %s", ca.ReasonCode, ca.Properties.ReasonString)
	}

	sa, err := m.mqtt.Subscribe(context.Background(), &paho.Subscribe{
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

	m.mqtt.Router.RegisterHandler(common.WorkflowExec, m.execMessage)

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

func (m *Mqtt) lostMQTT(err error) {
	log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
	time.Sleep(1 * time.Second)
	if err := m.mqtt.Disconnect(&paho.Disconnect{ReasonCode: 0}); err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Reconnecting..")
	}
	time.Sleep(1 * time.Second)
	_ = m.conMQTT()
}

func (m *Mqtt) execMessage(p *paho.Publish) {
	//var User = m.Properties.User
	//var CorrelationData = m.Properties.CorrelationData
	//var ResponseTopic = m.Properties.ResponseTopic

	log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", string(p.Payload), p.Topic)
	id := "false"
	s := strings.Split(p.Topic, "/")
	pl := string(p.Payload)

	if s[0] == common.ExtPrefix && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	log.Debug().Str("source", "MQTT").Msgf("Topic parser: %s\n", id)

	if id != "false" {
		cmd := exec.Command("/opt/wfexec/"+id+".sh", pl, common.EP)
		cmd.Dir = "/opt/wfexec/"
		_, err := cmd.CombinedOutput()

		if id == "sync" || id == "storage" || common.EP == "wf-srv" {
			notifyMessage(p.Payload)
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
	var file wf.Files

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

func (m *Mqtt) SendRespond(id string, p *MqttPayload) {
	var topic string

	if id == "false" {
		topic = common.ServiceDataTopic + common.EP
	} else {
		topic = common.ServiceDataTopic + common.EP + "/" + id
	}
	message, err := json.Marshal(p)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Message parsing")
	}

	pa, err := m.mqtt.Publish(context.Background(), &paho.Publish{
		QoS:     byte(1),
		Retain:  false,
		Topic:   topic,
		Payload: message,
	})
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msgf("MQTT Publish error: %d ", pa.Properties.ReasonString)
	}
}

func (m *Mqtt) SendMessage(source string, message []byte) {
	var topic string

	switch source {
	case "upload":
		topic = common.MonitorUploadTopic
	case "convert":
		topic = common.MonitorConvertTopic
	case "storage":
		topic = "exec/wf/storage/sync"
	}

	pa, err := m.mqtt.Publish(context.Background(), &paho.Publish{
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
