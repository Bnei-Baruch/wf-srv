package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Bnei-Baruch/wf-srv/common"
	"github.com/coreos/go-oidc"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"net/http"
)

type App struct {
	Router        *mux.Router
	tokenVerifier *oidc.IDTokenVerifier
	Msg           mqtt.Client
}

func (a *App) InitClient() {
	oidcProvider, err := oidc.NewProvider(context.TODO(), common.ACC_URL)
	if err != nil {
		log.Fatal().Str("source", "APP").Err(err).Msg("oidc.NewProvider")
	}
	a.tokenVerifier = oidcProvider.Verifier(&oidc.Config{
		SkipClientIDCheck: true,
	})
}

func (a *App) Initialize() {
	InitLog()
	log.Info().Str("source", "APP").Msg("initializing app")
	a.Router = mux.NewRouter()
	a.InitializeRoutes()
	a.initMQTT()
}

func (a *App) Run(port string) {
	headersOk := handlers.AllowedHeaders([]string{"X-Requested-With", "Content-Type", "Content-Length", "Accept-Encoding", "Content-Range", "Content-Disposition", "Authorization"})
	originsOk := handlers.AllowedOrigins([]string{"*"})
	methodsOk := handlers.AllowedMethods([]string{"GET", "DELETE", "POST", "PUT", "OPTIONS"})

	if port == "" {
		port = ":8010"
	}

	log.Info().Str("source", "APP").Msgf("app run %s", port)
	if err := http.ListenAndServe(port, handlers.CORS(originsOk, headersOk, methodsOk)(a.Router)); err != nil {
		log.Fatal().Str("source", "APP").Err(err).Msg("http.ListenAndServe")
	}
}

func (a *App) InitializeRoutes() {
	a.Router.Use(a.LoggingMiddleware)
	a.Router.HandleFunc("/convert", a.convertExec).Methods("GET")
	a.Router.HandleFunc("/{ep}/upload", a.handleUpload).Methods("POST")
	a.Router.HandleFunc("/workflow/{ep}", a.putJson).Methods("PUT")
	a.Router.HandleFunc("/file/save", a.saveFile).Methods("PUT")
	a.Router.HandleFunc("/{ep}/status", a.statusJson).Methods("GET")
	a.Router.HandleFunc("/convert/monitor", a.convertMonitor).Methods("GET")
	a.Router.HandleFunc("/upload/monitor", a.uploadMonitor).Methods("GET")
	if common.EP == "wf-srv" || common.EP == "wf-nas" {
		a.Router.PathPrefix("/backup/").Handler(http.StripPrefix("/backup/", http.FileServer(http.Dir("/backup"))))
		a.Router.PathPrefix("/mnt/").Handler(http.StripPrefix("/mnt/", http.FileServer(http.Dir("/mnt"))))
	} else {
		a.Router.PathPrefix("/ffconv/").Handler(http.StripPrefix("/ffconv/", http.FileServer(http.Dir("/ffconv"))))
	}
}

func (a *App) initMQTT() {
	if common.SERVER != "" {
		//a.InitLogMQTT()
		opts := mqtt.NewClientOptions()
		opts.AddBroker(fmt.Sprintf("ssl://%s", common.SERVER))
		opts.SetClientID(common.EP + "-exec_mqtt_client")
		opts.SetUsername(common.USERNAME)
		opts.SetPassword(common.PASSWORD)
		opts.SetAutoReconnect(true)
		opts.SetOnConnectHandler(a.SubMQTT)
		opts.SetConnectionLostHandler(a.LostMQTT)
		a.Msg = mqtt.NewClient(opts)
		if token := a.Msg.Connect(); token.Wait() && token.Error() != nil {
			err := token.Error()
			log.Fatal().Str("source", "MQTT").Err(err).Msg("initialize mqtt listener")
		}
	}
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}
