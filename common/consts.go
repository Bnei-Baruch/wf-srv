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

	CapPath = os.Getenv("CAP_PATH")
	LogPath = os.Getenv("LOG_PATH")

	ExecTopic     = "workflow/exec/#"
	WorkflowTopic = "workflow/server/#"
)

const (
	ExtPrefix           = "kli/"
	ServiceDataTopic    = "workflow/exec/data/"
	WorkflowDataTopic   = "workflow/server/data/"
	MonitorUploadTopic  = "workflow/server/upload/monitor"
	MonitorConvertTopic = "workflow/server/convert/monitor"
)
