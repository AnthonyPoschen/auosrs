package main

import (
	"cmp"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/anthonyposchen/auosrs.git/templates"
	"github.com/anthonyposchen/auosrs.git/templates/route"
	"golang.org/x/time/rate"
)

const (
	clanID = 276
)

var Bosses = []string{"abyssal_sire", "alchemical_hydra", "artio", "barrows_chests", "bryophyta", "callisto", "calvarion", "cerberus", "chambers_of_xeric", "chambers_of_xeric_challenge_mode", "chaos_elemental", "chaos_fanatic", "commander_zilyana", "corporeal_beast", "crazy_archaeologist", "dagannoth_prime", "dagannoth_rex", "dagannoth_supreme", "deranged_archaeologist", "duke_sucellus", "general_graardor", "giant_mole", "grotesque_guardians", "hespori", "kalphite_queen", "king_black_dragon", "kraken", "kreearra", "kril_tsutsaroth", "mimic", "nex", "nightmare", "phosanis_nightmare", "obor", "phantom_muspah", "sarachnis", "scorpia", "scurrius", "skotizo", "spindel", "tempoross", "the_gauntlet", "the_corrupted_gauntlet", "the_leviathan", "the_whisperer", "theatre_of_blood", "theatre_of_blood_hard_mode", "thermonuclear_smoke_devil", "tombs_of_amascut", "tombs_of_amascut_expert", "tzkal_zuk", "tztok_jad", "vardorvis", "venenatis", "vetion", "vorkath", "wintertodt", "zalcano", "zulrah"}

type ClanBossInfo struct {
	// map of boss name to the amount of kills in 1 week
	Bosses map[string]int
	sync.Mutex
}

var clanBossInfo = ClanBossInfo{Mutex: sync.Mutex{}, Bosses: make(map[string]int)}

//go:embed dist
var dist embed.FS

func getFileSystem() http.FileSystem {
	fsys, err := fs.Sub(dist, "dist")
	if err != nil {
		log.Fatal(err)
	}
	return http.FS(fsys)
}

func main() {
	go wiseOldmanSync()
	// start hourly cron to update wiseold man information
	http.HandleFunc("/activity", Activity)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// for specifically the root / index
		// we want to replace the default with our own
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			templates.Index().Render(context.Background(), w)
			return
			// do a templ include!!
		}
		http.FileServer(getFileSystem()).ServeHTTP(w, r)
	})
	fmt.Println("Server is running on port 42069")
	fmt.Println(http.ListenAndServe(":42069", nil))
}

func Activity(w http.ResponseWriter, r *http.Request) {
	clanBossInfo.Lock()
	defer clanBossInfo.Unlock()
	var Data []route.ActivityBoss
	for k, v := range clanBossInfo.Bosses {
		Data = append(Data, route.ActivityBoss{Name: strings.ReplaceAll(k, "_", " "), Kills: v})
	}
	slices.SortFunc(Data, func(i route.ActivityBoss, j route.ActivityBoss) int {
		return cmp.Compare(i.Kills, j.Kills) * -1
	})
	route.Activity(Data).Render(context.Background(), w)
}

type wiseOldMangained struct {
	Data struct {
		Gained int `json:"gained"`
	} `json:"data"`
}

func wiseOldmanSync() {
	rl := rate.NewLimiter(rate.Every(time.Minute/20), 1)
	Client := new(http.Client)
	Client.Timeout = 10 * time.Second

	api := "https://api.wiseoldman.net/v2/groups/%d/gained?metric=%s&period=week&limit=50&offset=%d"
	for {
		t := time.After(1 * time.Hour)
		// lookup wiseoldman information per boss
		for _, boss := range Bosses {
			offset := 0
			bossResults := make([]wiseOldMangained, 0)
			// loop for the boss until the clan hasn't gained any more kills
			for {
				rl.Wait(context.Background())
				fmt.Println("fetching: Boss: ", boss, "Offset: ", offset)
				resp, err := http.Get(fmt.Sprintf(api, clanID, boss, offset))
				if err != nil {
					log.Println(err)
					break
				}
				defer resp.Body.Close()
				results := make([]wiseOldMangained, 0)
				data, err := io.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					break
				}
				json.Unmarshal(data, &results)
				if len(results) == 0 {
					break
				}
				bossResults = append(bossResults, results...)
				if results[len(results)-1].Data.Gained == 0 {
					break
				}
				offset += 50
			}
			var total int
			for _, v := range bossResults {
				total += v.Data.Gained
			}
			fmt.Println("Boss: ", boss, "Total: ", total)
			// replace the value in the mutex data
			clanBossInfo.Lock()
			clanBossInfo.Bosses[boss] = total
			clanBossInfo.Unlock()
		}
		// make two tables for bosses for 7 days and 30 days
		//
		fmt.Println("fetching completed")
		<-t
	}
}
