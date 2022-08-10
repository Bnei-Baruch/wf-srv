package api

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/Bnei-Baruch/wf-srv/wf"
	"github.com/gabriel-vasile/mimetype"
	"github.com/rs/zerolog/log"
	"gopkg.in/vansante/go-ffprobe.v2"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

type File struct {
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size"`
	//HSize    string  `json:"h-size"`
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Children []*File `json:"children"`
}

type Mail struct {
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
	To      []string `json:"to"`
}

func JsonFilesTree(path string) interface{} {
	rootOSFile, _ := os.Stat(path)
	rootFile := toFile(rootOSFile, path)
	stack := []*File{rootFile}

	for len(stack) > 0 {
		file := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		children, _ := ioutil.ReadDir(file.Path)
		for _, ch := range children {
			child := toFile(ch, filepath.Join(file.Path, ch.Name()))
			file.Children = append(file.Children, child)
			stack = append(stack, child)
		}
		sort.Slice(file.Children[:], func(i, j int) bool {
			return file.Children[i].IsDir
		})

		sort.Slice(file.Children, func(i, j int) bool {
			return file.Children[i].Name < file.Children[j].Name
		})
	}

	return rootFile
}

func Round(val float64, roundOn float64, places int) (newVal float64) {
	var round float64
	pow := math.Pow(10, float64(places))
	digit := pow * val
	_, div := math.Modf(digit)
	if div >= roundOn {
		round = math.Ceil(digit)
	} else {
		round = math.Floor(digit)
	}
	newVal = round / pow
	return
}

func HumanFileSize(size float64) string {
	var suffixes [5]string
	suffixes[0] = "B"
	suffixes[1] = "KB"
	suffixes[2] = "MB"
	suffixes[3] = "GB"
	suffixes[4] = "TB"

	base := math.Log(size) / math.Log(1024)
	getSize := Round(math.Pow(1024, base-math.Floor(base)), .5, 2)
	getSuffix := suffixes[int(math.Floor(base))]
	return strconv.FormatFloat(getSize, 'f', -1, 64) + " " + getSuffix
}

func toFile(file os.FileInfo, path string) *File {
	JSONFile := File{
		ModTime: file.ModTime().Format("2006-01-02 15:04:05"),
		IsDir:   file.IsDir(),
		Size:    file.Size(),
		//HSize:   HumanFileSize(float64(file.Size())),
		Name: file.Name(),
		Path: path,
	}
	return &JSONFile
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
	files := make([]wf.Files, 0)

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

func (m *Mail) NotifyByMail() error {
	log.Debug().Str("source", "MAIL").Msgf("Notify by mail..\n")

	user := common.MAIL_USER
	password := common.MAIL_PASS
	smtpHost := common.MAIL_HOST
	from := common.MAIL_FROM
	smtpPort := "587"

	msg := []byte("From: " + from + "\r\n" + "Subject: " + m.Subject + "\r\n" + "\r\n" + "WF Notification: " + m.Body + "\r\n")
	auth := smtp.PlainAuth("", user, password, smtpHost)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, m.To, msg)
	if err != nil {
		log.Error().Str("source", "MAIL").Err(err).Msg("Notify by mail error")
		return err
	} else {
		log.Debug().Str("source", "MAIL").Msg("Notify by mail success.\n")
		return nil
	}
}
