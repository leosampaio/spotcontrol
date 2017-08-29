package main

import (
    "fmt"

    "log"
    "net/http"
    "flag"
    "os"
    "strings"
    "encoding/json"

    "github.com/leosampaio/spotcontrol"
    "github.com/gorilla/mux"
)

const defaultdevicename = "SpotControlServer"
const defaultcontrolleddevice = "SpotControl"
var sController *spotcontrol.SpircController
var controlledDeviceName *string

type UserRequest struct {
    Id  string `json:"id"`
}

type TracklistRequest struct {
    Ids  []string `json:"ids"`
}

type UserCommand struct {
    Command  string `json:"command"`
}

func main() {

    username := flag.String("username", "", "spotify username")
    password := flag.String("password", "", "spotify password")
    blobPath := flag.String("blobPath", "", "path to saved blob")
    devicename := flag.String("devicename", defaultdevicename, "name of device")
    controlledDeviceName = flag.String("controlled", defaultcontrolleddevice, "name of controlled device")
    flag.Parse()

    var err error
    if *username != "" && *password != "" {
        sController, err = spotcontrol.Login(*username, *password, *devicename)
    } else if *blobPath != "" {
        if _, err = os.Stat(*blobPath); os.IsNotExist(err) {
            sController, err = spotcontrol.LoginDiscovery(*blobPath, *devicename)
        } else {
            sController, err = spotcontrol.LoginBlobFile(*blobPath, *devicename)
        }
    } else if os.Getenv("client_secret") != "" {
        sController, err = spotcontrol.LoginOauth(*devicename)
    } else {
        fmt.Println("./server-controller --username SPOTIFY_USERNAME --password SPOTIFY_PASSWORD --devicename DEVICE_NAME")
        return
    }

    if err != nil {
        fmt.Println("Error logging in: ", err)
        return
    }

    sController.SendHello()

    router := mux.NewRouter()
    router.HandleFunc("/", Index)
    router.HandleFunc("/track", PlayTrack).Methods("POST")
    router.HandleFunc("/tracks", PlayTracks).Methods("POST")
    router.HandleFunc("/playlists", PlayPlaylist).Methods("POST")
    router.HandleFunc("/playlists", GetPlaylists).Methods("GET")
    router.HandleFunc("/command", ExecuteCommand).Methods("POST")

    log.Fatal(http.ListenAndServe(":8080", router))
}

func getDevice() *spotcontrol.ConnectDevice {
    devices := sController.ListDevices()
    if len(devices) == 0 {
        fmt.Println("Could not find device!")
    }

    for _, d := range devices {
        if d.Name == *controlledDeviceName {
            fmt.Printf("Found %v: %v\n", d.Name, d.Ident)
            return &d
        }
    }

    return new(spotcontrol.ConnectDevice)
}

func Index(w http.ResponseWriter, r *http.Request) {
    var device = getDevice()
    fmt.Fprintf(w, "Found %v: %v\n", device.Name, device.Ident)
}

func PlayTrack(w http.ResponseWriter, r *http.Request) {
    var ur UserRequest
    decoder := json.NewDecoder(r.Body)
    decoder.Decode(&ur)

    id := strings.TrimPrefix(ur.Id, "spotify:track:")
    trackArray := []string{id}

    var device = getDevice()
    sController.LoadTrack(device.Ident, trackArray)

    sController.SendPlay(device.Ident)

    status := map[string]string {
        "status": "sucess",
    }

    json.NewEncoder(w).Encode(status)
}

func PlayTracks(w http.ResponseWriter, r *http.Request) {
    var tr TracklistRequest
    decoder := json.NewDecoder(r.Body)
    decoder.Decode(&tr)

    items := tr.Ids
    var ids []string
    for i := 0; i < len(items); i++ {
        id := strings.TrimPrefix(items[i], "spotify:track:")
        ids = append(ids, id)
    }

    var device = getDevice()
    sController.LoadTrack(device.Ident, ids)

    sController.SendPlay(device.Ident)

    status := map[string]string {
        "status": "sucess",
    }

    json.NewEncoder(w).Encode(status)
}

func PlayPlaylist(w http.ResponseWriter, r *http.Request) {

    var ur UserRequest
    decoder := json.NewDecoder(r.Body)
    decoder.Decode(&ur)

    id := strings.TrimPrefix(ur.Id, "spotify:")
    id = strings.Replace(id, ":", "/", -1)
    
    playlist, err := sController.GetPlaylist(id)
    if err != nil || playlist.Contents == nil {
        fmt.Println("Playlist not found")
        w.WriteHeader(http.StatusNotFound)
        w.Write([]byte("404 - Could not find playlist"))
        return 
    }

    items := playlist.Contents.Items
    var ids []string
    for i := 0; i < len(items); i++ {
        id := strings.TrimPrefix(items[i].GetUri(), "spotify:track:")
        ids = append(ids, id)
    }

    var device = getDevice()
    sController.LoadTrack(device.Ident, ids)

    sController.SendPlay(device.Ident)

    status := map[string]string {
        "status": "sucess",
    }

    json.NewEncoder(w).Encode(status)
}

func GetPlaylists(w http.ResponseWriter, r *http.Request) {
    playlist, _ := sController.GetRootPlaylist()

    var err error
    if err != nil || playlist.Contents == nil {
        w.WriteHeader(http.StatusNotFound)
        w.Write([]byte("404 - Could not find root list"))
        fmt.Println("Error getting root list")
    } else {
        items := playlist.Contents.Items
        playlistMap := make([]map[string]string, len(items))
        for i := 0; i < len(items); i++ {
            id := strings.TrimPrefix(items[i].GetUri(), "spotify:")
            id = strings.Replace(id, ":", "/", -1)
            list, _ := sController.GetPlaylist(id)

            playlistMap[i] = map[string]string {
                "id": id,
                "name": list.Attributes.GetName(),
            }
        }
        json.NewEncoder(w).Encode(playlistMap)
    }
}

func ExecuteCommand(w http.ResponseWriter, r *http.Request) {

    var uc UserCommand
    decoder := json.NewDecoder(r.Body)
    decoder.Decode(&uc)

    var device = getDevice()

    switch {
        case uc.Command == "play":
            sController.SendPlay(device.Ident)
        case uc.Command == "pause":
            sController.SendPause(device.Ident)
        case uc.Command == "next":
            sController.SendNext(device.Ident)
        case uc.Command == "prev":
            sController.SendPrev(device.Ident)
    }

    status := map[string]string {
        "status": "sucess",
    }

    json.NewEncoder(w).Encode(status)
}
