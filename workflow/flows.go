package workflow

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
)

type MqttWorkflow struct {
	Action  string      `json:"action,omitempty"`
	ID      string      `json:"id,omitempty"`
	Name    string      `json:"name,omitempty"`
	Source  string      `json:"src,omitempty"`
	Error   error       `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Result  string      `json:"result,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func MqttMessage(c mqtt.Client, m mqtt.Message) {
	log.Debug().Str("source", "MQTT").RawJSON("json", m.Payload()).Msg("MqttMessage: Topic - " + m.Topic())
	id := "false"
	s := strings.Split(m.Topic(), "/")

	if s[0] == "kli" && len(s) == 5 {
		id = s[4]
	} else if s[0] == "workflow" && len(s) == 4 {
		id = s[3]
	}

	mp := MqttWorkflow{}
	err := json.Unmarshal(m.Payload(), &mp)
	if err != nil {
		log.Error().Str("source", "MQTT").Err(err).Msg("Unmarshal")
	}

	if id != "false" {
		switch mp.Action {
		case "start":
			go StartFlow(mp, c)
		case "line":
			go LineFlow(mp, c)
		case "stop":
			go StopFlow(mp, c)
		}
	}
}

func Publish(topic string, message string, c mqtt.Client) {
	text := fmt.Sprintf(message)
	log.Debug().Str("source", "MQTT").Str("json", message).Msg("Publish: Topic - " + topic)
	if token := c.Publish(topic, byte(1), false, text); token.Wait() && token.Error() != nil {
		log.Error().Str("source", "MQTT").Err(token.Error()).Msg("Publish: Topic - " + topic)
	}
}

func StartFlow(rp MqttWorkflow, c mqtt.Client) {

	src := common.EP
	ep := "/ingest/"

	if src == "archcap" {
		ep = "/capture/"
	}

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		log.Error().Str("source", "CAP").Err(rp.Error).Msg("StartFlow: CaptureID is empty")
		rp.Message = "Internal error"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	cm := &MdbPayload{
		CaptureSource: src,
		Station:       GetStationID(src),
		User:          "operator@dev.com",
		FileName:      cs.StartName,
		WorkflowID:    rp.ID,
	}

	err := cm.PostMDB("capture_start")
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StartFlow: PostMDB")
		rp.Error = err
		rp.Message = "MDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	ws := &Wfstatus{Capwf: false, Trimmed: false, Sirtutim: false}
	cw := &WfdbCapture{
		CaptureID: rp.ID,
		Date:      GetDateFromID(rp.ID),
		StartName: cs.StartName,
		CapSrc:    src,
		Wfstatus:  *ws,
	}

	err = cw.PutWFDB(rp.Action, ep)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StartFlow: PutWFDB")
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	rp.Message = "Success"
	m, _ := json.Marshal(rp)

	Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
}

func LineFlow(rp MqttWorkflow, c mqtt.Client) {

	src := common.EP
	ep := "/ingest/"

	if src == "archcap" {
		ep = "/capture/"
	}

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		log.Error().Str("source", "CAP").Err(rp.Error).Msg("LineFlow: CaptureID is empty")
		rp.Message = "Internal error"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	ws := &Wfstatus{Capwf: false, Trimmed: false, Sirtutim: false}
	cw := &WfdbCapture{
		CaptureID: rp.ID,
		Date:      GetDateFromID(rp.ID),
		StartName: cs.StartName,
		CapSrc:    src,
		Wfstatus:  *ws,
		Line:      cs.Line,
	}

	err := cw.PutWFDB(rp.Action, ep)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("LineFlow: PutWFDB")
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	rp.Message = "Success"
	m, _ := json.Marshal(rp)

	Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
}

func StopFlow(rp MqttWorkflow, c mqtt.Client) {

	src := common.EP
	ep := "/ingest/"

	cs := GetState()
	if cs.CaptureID == "" {
		rp.Error = fmt.Errorf("error")
		log.Error().Str("source", "CAP").Err(rp.Error).Msg("StopFlow: CaptureID is empty")
		rp.Message = "Internal error"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	StopName := cs.StopName
	if src == "archcap" {
		StopName = strings.Replace(StopName, "_o_", "_s_", 1)
	}

	file, err := os.Open(common.CapPath + rp.ID + ".mp4")
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: Error open file")
		return
	}

	stat, err := file.Stat()
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: Error get stat file")
		return
	}

	size := stat.Size()
	log.Debug().Str("source", "CAP").Msgf("stopFlow file size: ", size)

	time := stat.Sys().(*syscall.Stat_t)
	//FIXME: WTF?
	ctime := time.Ctimespec.Nsec //OSX
	//ctime := time.Ctim.Nsec //Linux
	log.Debug().Str("source", "CAP").Msgf("Creation time file: ", ctime)

	h := sha1.New()
	if _, err = io.Copy(h, file); err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: Filed to get sha1")
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	log.Debug().Str("source", "CAP").Msgf("stopFlow file sha1: ", sha)

	cm := &MdbPayload{
		CaptureSource: src,
		Station:       GetStationID(src),
		User:          "operator@dev.com",
		FileName:      StopName,
		WorkflowID:    rp.ID,
		CreatedAt:     ctime,
		Size:          size,
		Sha:           sha,
		Part:          "false",
	}

	cw := &WfdbCapture{}
	err = cw.GetWFDB(rp.ID)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: GetWFDB")
		return
	}

	cw.Sha1 = sha
	cw.StopName = StopName

	//Main Multi Capture
	if src == "mltcap" || src == "maincap" {
		if cw.Line.ContentType == "LESSON_PART" {
			cm.Part = strconv.Itoa(cw.Line.Part)
			cm.LessonID = cw.Line.LessonID
		}
	}

	//Archive Source Capture
	if src == "archcap" {
		cw.CapSrc = "archcap"
		if cw.Line.ContentType == "LESSON_PART" {
			cm.Part = strconv.Itoa(cw.Line.Part)
			cm.LessonID = cw.Line.LessonID
		}
		ep = "/capture/"
	}

	//Backup Multi Capture
	if src == "mltbackup" || src == "backupcap" {
		if cw.Line.ContentType == "LESSON_PART" {
			StopName = StopName[:len(StopName)-2] + "full"
			cw.Line.ContentType = "FULL_LESSON"
			cw.Line.Part = -1
			cw.Line.FinalName = StopName
			cw.StopName = StopName
			cm.Part = "full"
			cm.LessonID = cw.Line.LessonID
		}
	}

	err = cw.PutWFDB(rp.Action, ep)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: PutWFDB")
		rp.Error = err
		rp.Message = "WFDB Request Failed"
		m, _ := json.Marshal(rp)
		Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
		return
	}

	err = cm.PostMDB("capture_stop")
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: PostMDB")
		return
	}

	FullName := StopName + "_" + rp.ID + ".mp4"
	err = os.Rename(common.CapPath+rp.ID+".mp4", common.CapPath+FullName)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: failed to rename file")
		return
	}

	cf := CaptureFlow{
		FileName:  FullName,
		Source:    "ingest",
		CapSrc:    src,
		CaptureID: rp.ID,
		Size:      size,
		Sha:       sha,
		Url:       "http://" + cm.Station + ":8080/get/" + FullName,
	}

	err = cf.PutFlow()
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("StopFlow: PutFlow")
		return
	}

	rp.Message = "Success"
	m, _ := json.Marshal(rp)

	Publish(common.WorkflowDataTopic+rp.Action, string(m), c)
}
