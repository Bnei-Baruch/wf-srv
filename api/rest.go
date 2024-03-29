package api

import (
	"encoding/json"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/Bnei-Baruch/wf-srv/wf"
	"github.com/gorilla/mux"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

func getUploadPath(ep string) string {

	switch ep {
	case "insert":
		return "/backup/tmp/insert/"
	case "jobs":
		return "/backup/jobs/"
	case "products":
		return "/backup/files/upload/"
	case "aricha":
		return "/backup/aricha/"
	case "aklada":
		return "/backup/tmp/akladot/"
	case "gibuy":
		return "/backup/tmp/gibuy/"
	case "carbon":
		return "/backup/tmp/carbon/"
	case "dgima":
		return "/backup/dgima/"
	case "proxy":
		return "/backup/tmp/proxy/"
	case "youtube":
		return "/backup/tmp/youtube/"
	case "coder":
		return "/backup/tmp/coder/"
	case "muxer":
		return "/backup/tmp/muxer/"
	default:
		return "/backup/tmp/upload/"
	}
}

func (a *App) getFile(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	file := vars["file"]
	lang := r.FormValue("audio")
	video := r.FormValue("video")

	req, err := http.NewRequest("GET", common.CdnUrl+"/"+file, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	//req.Header.Set("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var data map[string]interface{}
	json.Unmarshal([]byte(body), &data)
	f := data["file_name"].(string)
	fileName := ""
	n := strings.Split(f, "_")
	n = n[1:]
	s := strings.Join(n, "_")

	if video == "" {
		fileName = lang + "_" + s + ".m4a"
	} else {
		fileName = lang + "_" + s + "_" + video + ".mp4"
	}

	if _, err := os.Stat(common.CachePath + fileName); os.IsNotExist(err) {
		cmdArguments := []string{f, lang, video}
		cmd := exec.Command("/opt/wfexec/remux.sh", cmdArguments...)
		cmd.Dir = "/opt/wfexec/"
		_, err := cmd.CombinedOutput()

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
	}

	http.Redirect(w, r, "https://files.kab.sh/muxed/"+fileName, 302)
}

func (a *App) getFileLocation(w http.ResponseWriter, r *http.Request) {

	vars := mux.Vars(r)
	file := vars["file"]
	nv := r.FormValue("no_video")

	req, err := http.NewRequest("GET", common.CdnUrl+"/"+file, nil)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
	}
	//req.Header.Set("Authorization", bearer)
	req.Header.Add("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
	}

	var data map[string]interface{}
	json.Unmarshal([]byte(body), &data)
	f := data["filename"].(string)
	n := strings.Split(f, "/")
	n = n[4:]
	s := strings.Join(n, "/")

	if nv == "true" {
		http.Redirect(w, r, "https://files.kab.sh/hls/tracks/a0/"+file+"/"+s+"/master.m3u8", 302)
	} else {
		http.Redirect(w, r, "https://files.kab.sh/hls/"+file+"/"+s+"/master.m3u8", 302)
	}
}

func (a *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	endpoint := vars["ep"]
	var u Upload

	uploadpath := getUploadPath(endpoint)

	if _, err := os.Stat(uploadpath); os.IsNotExist(err) {
		os.MkdirAll(uploadpath, 0755)
	}

	var n int
	var err error

	// define pointers for the multipart reader and its parts
	var mr *multipart.Reader
	var part *multipart.Part

	//log.Println("File Upload Endpoint Hit")

	if mr, err = r.MultipartReader(); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// buffer to be used for reading bytes from files
	chunk := make([]byte, 10485760)

	// continue looping through all parts, *multipart.Reader.NextPart() will
	// return an End of File when all parts have been read.
	for {
		// variables used in this loop only
		// tempfile: filehandler for the temporary file
		// filesize: how many bytes where written to the tempfile
		// uploaded: boolean to flip when the end of a part is reached
		var tempfile *os.File
		var filesize int
		var uploaded bool

		if part, err = mr.NextPart(); err != nil {
			if err != io.EOF {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				os.Remove(tempfile.Name())
			} else {
				respondWithJSON(w, http.StatusOK, u)
			}
			return
		}
		// at this point the filename and the mimetype is known
		//log.Printf("Uploaded filename: %s", part.FileName())
		//log.Printf("Uploaded mimetype: %s", part.Header)

		u.Filename = part.FileName()
		u.Mimetype = part.Header.Get("Content-Type")
		u.Url = uploadpath + u.Filename

		tempfile, err = ioutil.TempFile(uploadpath, part.FileName()+".*")
		if err != nil {
			respondWithError(w, http.StatusInternalServerError, err.Error())
			return
		}
		defer tempfile.Close()
		//defer os.Remove(tempfile.Name())

		// continue reading until the whole file is upload or an error is reached
		for !uploaded {
			if n, err = part.Read(chunk); err != nil {
				if err != io.EOF {
					respondWithError(w, http.StatusInternalServerError, err.Error())
					os.Remove(tempfile.Name())
					return
				}
				uploaded = true
			}

			if n, err = tempfile.Write(chunk[:n]); err != nil {
				respondWithError(w, http.StatusInternalServerError, err.Error())
				os.Remove(tempfile.Name())
				return
			}
			filesize += n
		}

		// once uploaded something can be done with the file, the last defer
		// statement will remove the file after the function returns so any
		// errors during upload won't hit this, but at least the tempfile is
		// cleaned up

		os.Rename(tempfile.Name(), u.Url)
		u.UploadProps(u.Url, endpoint)
	}
}

func (a *App) saveFile(w http.ResponseWriter, r *http.Request) {
	var f wf.Files

	d := json.NewDecoder(r.Body)
	if err := d.Decode(&f); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
		return
	}

	defer r.Body.Close()

	if err := f.SaveFile(); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	message, _ := json.Marshal(f)
	go a.SendMessage("storage", message)

	respondWithJSON(w, http.StatusOK, f)
}

func (a *App) putJson(w http.ResponseWriter, r *http.Request) {
	var s Status
	vars := mux.Vars(r)
	endpoint := vars["ep"]

	b, _ := ioutil.ReadAll(r.Body)

	err := s.PutExec(endpoint, string(b))

	defer r.Body.Close()

	if err != nil {
		s.Status = "error"
	} else {
		s.Status = "ok"
	}

	respondWithJSON(w, http.StatusOK, s)
}

func (a *App) statusJson(w http.ResponseWriter, r *http.Request) {
	var s Status
	vars := mux.Vars(r)
	endpoint := vars["ep"]
	id := r.FormValue("id")
	key := r.FormValue("key")
	value := r.FormValue("value")

	err := s.GetStatus(endpoint, id, key, value)

	if err != nil {
		s.Status = "error"
	} else {
		s.Status = "ok"
	}

	respondWithJSON(w, http.StatusOK, s)
}

func (a *App) getFilesTree(w http.ResponseWriter, r *http.Request) {

	tf := JsonFilesTree(common.TreePath)

	respondWithJSON(w, http.StatusOK, tf)
}

func (a *App) sendMail(w http.ResponseWriter, r *http.Request) {
	var s Status
	subject := r.FormValue("subject")
	body := r.FormValue("body")

	SendEmail(subject, body)
	s.Status = "ok"

	respondWithJSON(w, http.StatusOK, s)
}

func (a *App) notifyByMail(w http.ResponseWriter, r *http.Request) {
	var m Mail

	d := json.NewDecoder(r.Body)
	if err := d.Decode(&m); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid resquest payload")
		return
	}

	defer r.Body.Close()

	if err := m.NotifyByMail(); err != nil {
		respondWithError(w, http.StatusInternalServerError, err.Error())
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) uploadMonitor(w http.ResponseWriter, r *http.Request) {
	cmdArguments := []string{}
	cmd := exec.Command("/opt/wfexec/get_upload.sh", cmdArguments...)
	cmd.Dir = "/opt/wfexec/"
	message, _ := cmd.CombinedOutput()

	go a.SendMessage("upload", message)

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) localUpload(w http.ResponseWriter, r *http.Request) {
	cmdArguments := []string{}
	cmd := exec.Command("/opt/wfexec/get_local.sh", cmdArguments...)
	cmd.Dir = "/opt/wfexec/"
	message, _ := cmd.CombinedOutput()

	go a.SendMessage("local", message)

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) convertMonitor(w http.ResponseWriter, r *http.Request) {
	cmdArguments := []string{}
	cmd := exec.Command("/opt/convert/status.sh", cmdArguments...)
	cmd.Dir = "/opt/convert/"
	message, _ := cmd.CombinedOutput()

	go a.SendMessage("convert", message)

	respondWithJSON(w, http.StatusOK, map[string]string{"result": "success"})
}

func (a *App) convertExec(w http.ResponseWriter, r *http.Request) {
	var s Status

	id := r.FormValue("id")
	key := r.FormValue("key")
	value := r.FormValue("value")

	err := s.GetExec(id, key, value)

	if err != nil {
		s.Status = "error"
	} else {
		s.Status = "ok"
	}

	respondWithJSON(w, http.StatusOK, s)
}
