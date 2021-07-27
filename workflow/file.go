package workflow

import (
	"errors"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/rs/zerolog/log"
	"mime"
	"os"
	"strconv"
	"strings"
	"time"
)

type Files struct {
	ID        int                    `json:"id"`
	FileID    string                 `json:"file_id"`
	Date      string                 `json:"date"`
	FileName  string                 `json:"file_name"`
	Extension string                 `json:"extension"`
	Size      int64                  `json:"size"`
	Sha1      string                 `json:"sha1"`
	FileType  string                 `json:"file_type"`
	Language  string                 `json:"language"`
	MimeType  string                 `json:"mime_type"`
	UID       string                 `json:"uid"`
	WID       string                 `json:"wid"`
	MediaInfo interface{}            `json:"media_info"`
	Props     map[string]interface{} `json:"properties"`
	ProductID string                 `json:"product_id"`
}

func (f *Files) SaveFile() error {

	TimeStamp := time.Now().UnixNano()
	DateNow := time.Now().Format("2006-01-02")
	Year := strings.Split(DateNow, "-")[0] + "/"
	Month := strings.Split(DateNow, "-")[1] + "/"
	FileId := "f" + strconv.FormatInt(TimeStamp, 10)
	FileName := f.FileName
	SavePath := common.FilesPath + Year + Month

	// Take extension by mime type
	var FileExt string
	if f.MimeType == "application/octet-stream" {
		FileExt = "srt"
	} else {
		e, err := mime.ExtensionsByType(f.MimeType)
		if err != nil {
			return err
		}
		if len(e) == 0 {
			err = errors.New("SaveFile: file type not recognized")
			return err
		}
		FileExt = strings.Trim(e[len(e)-1], ".")
	}

	// Make directory
	if _, err := os.Stat(SavePath); os.IsNotExist(err) {
		err = os.Mkdir(SavePath, os.ModeDir|0755)
		log.Error().Str("source", "FILES").Err(err).Msg("SaveFile: failed to make directory")
		return err
	}

	FilePath := SavePath + FileName + "_" + FileId + "." + FileExt

	f.FileID = FileId
	f.Date = DateNow
	f.Extension = FileExt
	f.Props["removed"] = false
	f.Props["timestamp"] = TimeStamp
	f.Props["url"] = FilePath

	err := os.Rename(common.UploadPath+f.Sha1, FilePath)
	if err != nil {
		log.Error().Str("source", "FILES").Err(err).Msg("SaveFile: failed to move file")
		return err
	}

	return nil
}
