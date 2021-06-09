package cmd

import (
	"flag"
	"github.com/Bnei-Baruch/wf-srv/api"
	"github.com/Bnei-Baruch/wf-srv/common"
)


func Init() {
	flag.Parse()
	a := api.App{}
	a.InitClient()
	a.Initialize()
	a.Run(":" + common.PORT)
}
