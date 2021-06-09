package api

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	wf "github.com/Bnei-Baruch/wf-srv/workflow"
	"github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
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
	if token := a.Msg.Subscribe(common.ExecTopic, byte(2), a.execMessage); token.Wait() && token.Error() != nil {
		log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	} else {
		log.Info().Str("source", "MQTT").Msg("Subscription - " + common.ExecTopic)
	}

	if token := a.Msg.Subscribe(common.ExtPrefix+common.ExecTopic, byte(2), a.execMessage); token.Wait() && token.Error() != nil {
		log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	} else {
		log.Info().Str("source", "MQTT").Msg("Subscription - " + common.ExtPrefix + common.ExecTopic)
	}

	//TODO: Will come to chenge old exec flow
	if token := a.Msg.Subscribe(common.WorkflowTopic, byte(2), wf.MqttMessage); token.Wait() && token.Error() != nil {
		log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	} else {
		log.Info().Str("source", "MQTT").Msg("Subscription - " + common.WorkflowTopic)
	}

	if token := a.Msg.Subscribe(common.ExtPrefix+common.WorkflowTopic, byte(2), wf.MqttMessage); token.Wait() && token.Error() != nil {
		log.Fatal().Str("source", "MQTT").Err(token.Error()).Msg("Subscription error")
	} else {
		log.Info().Str("source", "MQTT").Msg("Subscription - " + common.ExtPrefix + common.WorkflowTopic)
	}
}

func (a *App) LostMQTT(c mqtt.Client, err error) {
	log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
}

func (a *App) execMessage(c mqtt.Client, m mqtt.Message) {
	log.Debug().Str("source", "MQTT").Msgf("Received message: %s from topic: %s\n", m.Payload(), m.Topic())
	id := "false"
	s := strings.Split(m.Topic(), "/")
	p := string(m.Payload())


	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "exec" && len(s) == 4 {
		id = s[3]
	}

	cmd := exec.Command("/opt/wfexec/" + id + ".sh", p)
	cmd.Dir = "/opt/wfexec/"
	_, err := cmd.CombinedOutput()

	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Lost Connection")
	}

	//s.Out = string(out)
	//json.Unmarshal(out, &s.Result)
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

	text := fmt.Sprintf(string(message))
	if token := a.Msg.Publish(topic, byte(2), false, text); token.Wait() && token.Error() != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Send Respond")
	}
}

func (a *App) SendMessage(id string) {
	var topic string
	var m interface{}
	//date := time.Now().Format("2006-01-02")

	if id == "ingest" {
		topic = common.MonitorUploadTopic
		//m, _ = models.FindIngest(a.DB, "date", date)
	}

	message, err := json.Marshal(m)
	if err != nil {
		log.Error().Str("monitor", "MQTT").Err(err).Msg("Message parsing")
	}

	text := fmt.Sprintf(string(message))
	if token := a.Msg.Publish(topic, byte(0), true, text); token.Wait() && token.Error() != nil {
		log.Error().Str("monitor", "MQTT").Err(err).Msg("Report Monitor")
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
