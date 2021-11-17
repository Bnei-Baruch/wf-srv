package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/Bnei-Baruch/wf-srv/workflow"
	"github.com/gabriel-vasile/mimetype"
	"github.com/rs/zerolog/log"
	"gopkg.in/vansante/go-ffprobe.v2"
	"io"
	"io/ioutil"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Upload struct {
	Filename  string      `json:"file_name"`
	Extension string      `json:"extension,omitempty"`
	Sha1      string      `json:"sha1"`
	Size      int64       `json:"size"`
	Mimetype  string      `json:"type"`
	Url       string      `json:"url"`
	MediaInfo interface{} `json:"media_info"`
}

type Status struct {
	Status string                 `json:"status"`
	Out    string                 `json:"stdout"`
	Result map[string]interface{} `json:"jsonst"`
}

func (s *Status) PutExec(endpoint string, p string) error {

	cmd := exec.Command("/opt/wfexec/"+endpoint+".sh", p)
	cmd.Dir = "/opt/wfexec/"
	out, err := cmd.CombinedOutput()

	if err != nil {
		s.Out = err.Error()
		return err
	}

	s.Out = string(out)
	json.Unmarshal(out, &s.Result)

	return nil
}

func (s *Status) GetExec(id string, key string, value string) error {

	cmdArguments := []string{id, key, value}
	cmd := exec.Command("/opt/convert/exec.sh", cmdArguments...)
	cmd.Dir = "/opt/convert"
	out, err := cmd.CombinedOutput()

	if err != nil {
		s.Out = err.Error()
		return err
	}

	s.Out = string(out)
	json.Unmarshal(out, &s.Result)

	return nil
}

func (s *Status) GetStatus(endpoint string, id string, key string, value string) error {

	cmdArguments := []string{id, key, value}
	cmd := exec.Command("/opt/wfexec/get_"+endpoint+".sh", cmdArguments...)
	cmd.Dir = "/opt/wfexec/"
	out, err := cmd.CombinedOutput()

	if err != nil {
		s.Out = err.Error()
		return err
	}

	s.Out = string(out)
	json.Unmarshal(out, &s.Result)

	return nil
}

func (u *Upload) UploadProps(filepath string, ep string) error {

	f, err := os.Open(filepath)
	if err != nil {
		return err
	}

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	u.Size = fi.Size()

	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}

	u.Sha1 = hex.EncodeToString(h.Sum(nil))

	if ep == "insert" {
		newpath := "/backup/tmp/insert/" + u.Sha1
		err = os.Rename(u.Url, newpath)
		if err != nil {
			return err
		}
		u.Url = newpath
	}

	if ep == "products" {
		newpath := "/backup/files/upload/" + u.Sha1
		err = os.Rename(u.Url, newpath)
		if err != nil {
			return err
		}
		u.Url = newpath

		mt, err := mimetype.DetectFile(newpath)
		if err != nil {
			return err
		}

		u.Mimetype = mt.String()

		if u.Mimetype == "application/octet-stream" {
			u.Extension = "srt"
		} else {
			u.Extension = strings.Trim(mt.Extension(), ".")
		}

		ctx, cancelFn := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelFn()

		data, err := ffprobe.ProbeURL(ctx, newpath)
		if err == nil {
			u.MediaInfo = data
		}
	}

	return nil
}

func IsExist(name string) (error, bool) {
	files := make([]workflow.Files, 0)

	var bearer = "Bearer " + common.PASSWORD
	req, err := http.NewRequest("GET", common.WfdbUrl+"?file_name="+name, nil)
	if err != nil {
		return err, true
	}
	req.Header.Set("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err, true
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err, true
	}

	//log.Debug().Str("source", "REQ").Msgf("File Name: %s \n", string(body))

	err = json.Unmarshal(body, &files)
	if err != nil {
		return err, true
	}

	log.Debug().Str("source", "REQ").Msgf("Respond length: %s \n", len(files))

	if len(files) > 1 {
		log.Debug().Str("source", "REQ").Msgf("Already exist: %s \n", len(files) > 0)
		return nil, true
	}

	return nil, false
}

func SendEmail(subject string, body string) {
	log.Debug().Str("source", "MAIL").Msgf("Sending mail..\n")

	o := strings.Split(subject, "_")[1]
	if o == "o" {
		l := strings.Split(subject, "_")[0]
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

		msg := []byte("From: " + from + "\r\n" + "Subject: " + subject + "\r\n" + "\r\n" + "Product ID: " + body + "\r\n")
		auth := smtp.PlainAuth("", user, password, smtpHost)
		err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, to, msg)
		if err != nil {
			log.Error().Str("source", "MAIL").Err(err).Msg("SendMail error")
		} else {
			log.Debug().Str("source", "MAIL").Msg("Mail sent.\n")
		}
	}
}
