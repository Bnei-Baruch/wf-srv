package common

import "os"

var (
	MltMain   = os.Getenv("MLT_MAIN")
	MltBackup = os.Getenv("MLT_BACKUP")
	MainCap   = os.Getenv("MAIN_CAP")
	BackupCap = os.Getenv("BACKUP_CAP")
	ArchCap   = os.Getenv("ARCH_CAP")

	PORT      = os.Getenv("LISTEN_ADDRESS")
	ACC_URL   = os.Getenv("ACC_URL")
	SKIP_AUTH = os.Getenv("SKIP_AUTH") == "true"

	SdbUrl   = os.Getenv("SDB_URL")
	WfApiUrl = os.Getenv("WFAPI_URL")
	MdbUrl   = os.Getenv("MDB_URL")
	WfdbUrl  = os.Getenv("WFDB_URL")

	EP       = os.Getenv("MQTT_EP")
	SERVER   = os.Getenv("MQTT_URL")
	USERNAME = os.Getenv("MQTT_USER")
	PASSWORD = os.Getenv("MQTT_PASS")

	MAIL_FROM = os.Getenv("MAIL_FROM")
	MAIL_HOST = os.Getenv("MAIL_HOST")
	MAIL_PORT = os.Getenv("MAIL_PORT")
	MAIL_USER = os.Getenv("MAIL_USER")
	MAIL_PASS = os.Getenv("MAIL_PASS")

	CapPath    = os.Getenv("CAP_PATH")
	LogPath    = os.Getenv("LOG_PATH")
	UploadPath = os.Getenv("UPLOAD_PATH")
	FilesPath  = os.Getenv("FILES_PATH")
	TreePath   = os.Getenv("TREE_PATH")

	ExecTopic     = "workflow/exec/#"
	WorkflowTopic = "workflow/server/#"
	WorkflowExec  = "exec/workflow/#"
	ExtPrefix     = os.Getenv("MQTT_FWD_PREFIX")
)

const (
	ServiceDataTopic    = "workflow/exec/data/"
	WorkflowDataTopic   = "workflow/server/data/"
	MonitorUploadTopic  = "workflow/server/upload/monitor"
	MonitorConvertTopic = "workflow/server/convert/monitor"
)
