package wf

import (
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/eclipse/paho.golang/paho"
	"github.com/rs/zerolog/log"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type MdbPayload struct {
	CaptureSource string `json:"capture_source"`
	Station       string `json:"station"`
	User          string `json:"user"`
	FileName      string `json:"file_name"`
	WorkflowID    string `json:"workflow_id"`
	CreatedAt     int64  `json:"created_at,omitempty"`
	LessonID      string `json:"collection_uid,omitempty"`
	Part          string `json:"part,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Sha           string `json:"sha1,omitempty"`
}

type Wfstatus struct {
	Capwf    bool `json:"capwf"`
	Trimmed  bool `json:"trimmed"`
	Sirtutim bool `json:"sirtutim"`
}

type CaptureState struct {
	Action    string `json:"action"`
	BackupID  string `json:"backup_id"`
	CaptureID string `json:"capture_id"`
	Date      string `json:"date"`
	IsRec     bool   `json:"isRec"`
	IsHag     bool   `json:"isHag"`
	LineID    string `json:"line_id"`
	NextPart  bool   `json:"next_part"`
	ReqDate   string `json:"req_date"`
	StartName string `json:"start_name"`
	StopName  string `json:"stop_name"`
	Line      Line   `json:"line"`
	NumPrt    NumPrt `json:"num_prt"`
}

type NumPrt struct {
	Lesson  int `json:"LESSON_PART"`
	Meal    int `json:"MEAL"`
	Friends int `json:"FRIENDS_GATHERING"`
	Unknown int `json:"UNKNOWN"`
	Part    int `json:"part"`
}

type WfdbCapture struct {
	CaptureID string                 `json:"capture_id"`
	CapSrc    string                 `json:"capture_src"`
	Date      string                 `json:"date"`
	StartName string                 `json:"start_name"`
	StopName  string                 `json:"stop_name"`
	Sha1      string                 `json:"sha1"`
	Line      Line                   `json:"line"`
	Original  map[string]interface{} `json:"original"`
	Proxy     map[string]interface{} `json:"proxy"`
	Wfstatus  Wfstatus               `json:"wfstatus"`
}

type Line struct {
	ArtifactType   string   `json:"artifact_type"`
	AutoName       string   `json:"auto_name"`
	CaptureDate    string   `json:"capture_date"`
	CollectionType string   `json:"collection_type"`
	CollectionUID  string   `json:"collection_uid,omitempty"`
	CollectionID   int      `json:"collection_id,omitempty"`
	ContentType    string   `json:"content_type"`
	Episode        string   `json:"episode,omitempty"`
	FinalName      string   `json:"final_name"`
	FilmDate       string   `json:"film_date,omitempty"`
	HasTranslation bool     `json:"has_translation"`
	Holiday        bool     `json:"holiday"`
	Language       string   `json:"language"`
	Lecturer       string   `json:"lecturer"`
	LessonID       string   `json:"lid"`
	ManualName     string   `json:"manual_name"`
	Number         int      `json:"number"`
	Part           int      `json:"part"`
	PartType       int      `json:"part_type,omitempty"`
	Major          *Major   `json:"major,omitempty"`
	Pattern        string   `json:"pattern"`
	RequireTest    bool     `json:"require_test"`
	Likutim        []string `json:"likutims,omitempty"`
	Sources        []string `json:"sources"`
	Tags           []string `json:"tags"`
}

type Major struct {
	Type string `json:"type" binding:"omitempty,eq=source|eq=tag|eq=likutim"`
	Idx  int    `json:"idx" binding:"omitempty,gte=0"`
}

type CaptureFlow struct {
	FileName  string `json:"file_name"`
	Source    string `json:"source"`
	CapSrc    string `json:"capture_src"`
	CaptureID string `json:"capture_id"`
	Size      int64  `json:"size"`
	Sha       string `json:"sha1"`
	Url       string `json:"url"`
}

var Data []byte

func GetState() *CaptureState {
	var cs *CaptureState
	err := json.Unmarshal(Data, &cs)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("get state")
	}
	u, _ := json.Marshal(cs)
	log.Debug().Str("source", "CAP").RawJSON("json", u).Msg("get state")
	return cs
}

func (w *WorkFlow) SetState(m *paho.Publish) {
	cs := &CaptureState{}
	err := json.Unmarshal(m.Payload, &cs)
	if err != nil {
		log.Error().Str("source", "CAP").Err(err).Msg("get state")
	}
	u, _ := json.Marshal(cs)
	log.Debug().Str("source", "CAP").RawJSON("json", u).Msg("set state")
	Data = m.Payload
}

func (m *MdbPayload) PostMDB(ep string) error {
	u, _ := json.Marshal(m)
	log.Info().Str("source", "WF").Str("action", ep).RawJSON("json", u).Msg("post to MDB")
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("POST", common.MdbUrl+ep, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func (w *WfdbCapture) GetWFDB(id string) error {
	req, err := http.NewRequest("GET", common.WfdbUrl+"/ingest/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	log.Debug().Str("source", "CAP").RawJSON("json", body).Msg("get WFDB")
	err = json.Unmarshal(body, &w)
	if err != nil {
		return err
	}

	return nil
}

func (w *WfdbCapture) GetIngestState(id string) error {
	req, err := http.NewRequest("GET", common.WfdbUrl+"/state/ingest/"+id, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	log.Debug().Str("source", "CAP").RawJSON("json", body).Msg("get WFDB")
	err = json.Unmarshal(body, &w)
	if err != nil {
		return err
	}

	return nil
}

func (w *WfdbCapture) PutWFDB(action string, ep string) error {
	u, _ := json.Marshal(w)
	log.Info().Str("source", "WF").Str("action", action).RawJSON("json", u).Msg("put to WFDB")
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", common.WfdbUrl+ep+w.CaptureID, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func (w *CaptureFlow) PutFlow() error {
	u, _ := json.Marshal(w)
	log.Info().Str("source", "WF").Str("action", "workflow").RawJSON("json", u).Msg("send to workflow")
	body := strings.NewReader(string(u))
	req, err := http.NewRequest("PUT", common.WfApiUrl, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status code: %s", strconv.Itoa(res.StatusCode))
		return err
	}

	return nil
}

func GetStationID(id string) string {
	switch id {
	case "mltcap":
		return common.MltMain
	case "mltbackup":
		return common.MltBackup
	case "maincap":
		return common.MainCap
	case "backupcap":
		return common.BackupCap
	case "archcap":
		return common.ArchCap
	}

	return ""
}

func GetDateFromID(id string) string {
	ts := strings.Split(id, "c")[1]
	tsInt, _ := strconv.ParseInt(ts, 10, 64)
	tsTime := time.Unix(0, tsInt*int64(time.Millisecond))
	return strings.Split(tsTime.String(), " ")[0]
}
