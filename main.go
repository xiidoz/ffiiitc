package main

import (
	"encoding/json"
	"ffiiitc/classifier"
	"ffiiitc/config"
	"ffiiitc/firefly"
	"log"
	"net/http"
	"strconv"
)

// structs to handle payload from new transaction web hook
type FireflyTrn struct {
	Id          int64  `json:"transaction_journal_id"`
	Description string `json:"description"`
	Category    string `json:"category_name"`
}

type FireFlyContent struct {
	Transactions []FireflyTrn `json:"transactions"`
}

type FireflyWebHook struct {
	Content FireFlyContent `json:"content"`
}

// http handler for new transaction
func HandleNewTransactionWebHook(c *classifier.TrnClassifier, f *firefly.FireFlyHttpClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// only allow post method
		if r.Method != http.MethodPost {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// decode payload
		decoder := json.NewDecoder(r.Body)
		var hookData FireflyWebHook
		err := decoder.Decode(&hookData)
		if err != nil {
			http.Error(w, "bad data", http.StatusBadRequest)
			return
		}

		// perform classification
		for _, trn := range hookData.Content.Transactions {
			log.Printf(
				"hook new trn: received (id: %v) (description: %s)",
				trn.Id,
				trn.Description,
			)
			cat := c.ClassifyTransaction(trn.Description)
			log.Printf("hook new trn: classified (id: %v) (category: %s)", trn.Id, cat)
			err = f.UpdateTransactionCategory(strconv.FormatInt(trn.Id, 10), cat)
			if err != nil {
				log.Printf("hook new trn: error updating (id: %v)", trn.Id)
				log.Println(err)
			}
			log.Printf("hook new trn: updated (id: %v)", trn.Id)

		}

		w.WriteHeader(http.StatusOK)
	}
}

// http handler for new transaction
func HandleUpdateTransactionWebHook(c *classifier.TrnClassifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// only allow post method
		if r.Method != http.MethodPost {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// decode payload
		decoder := json.NewDecoder(r.Body)
		var hookData FireflyWebHook
		err := decoder.Decode(&hookData)
		if err != nil {
			http.Error(w, "bad data", http.StatusBadRequest)
			return
		}

		// perform classification
		for _, trn := range hookData.Content.Transactions {
			log.Printf(
				"hook update trn: received (id: %v) (desc: %s) (cat: %s)",
				trn.Id,
				trn.Description,
				trn.Category,
			)

			err := c.Train(trn.Description, trn.Category)
			if err != nil {
				log.Printf("hook update trn: error updating model: %v", err)
			}
			log.Printf("hook update trn: updated model for (cat: %s)", trn.Category)

		}
		w.WriteHeader(http.StatusOK)
	}
}

// wrapper to log http requests
func logRequest(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s\n", r.RemoteAddr, r.Method, r.URL)
		handler.ServeHTTP(w, r)
	})
}

func main() {

	// firefly app timeout (sec)
	const ffAppTimeout = 10

	// get the config
	cfg, err := config.NewMDConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	// make firefly http client for rest api
	fc := firefly.NewFireFlyHttpClient(cfg.FFApp, cfg.APIKey, ffAppTimeout)

	// make classifier
	// on first run, classifier will take all your
	// transactions and learn their categories
	// subsequent start classifier will load trained model from file
	cls := classifier.NewTrnClassifier(fc)
	log.Printf("Learned classes:\n %v", cls.Classifier.Classes)

	http.HandleFunc("/", HandleNewTransactionWebHook(cls, fc))
	http.HandleFunc("/learn", HandleUpdateTransactionWebHook(cls))
	log.Fatal(http.ListenAndServe(":8080", logRequest(http.DefaultServeMux)))
}
