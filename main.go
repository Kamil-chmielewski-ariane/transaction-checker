package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/hashgraph/hedera-sdk-go/v2"
)

type TransactionPayload struct {
	TxID                    string  `json:"transactionId"`
	TxType                  string  `json:"type"`
	BlockNumber             int     `json:"blockNumber"`
	AddressTo               string  `json:"addressTo"`
	TxTimestamp             string  `json:"txTimestamp"`
	Timestamp               string  `json:"currentTimestamp"`
	EthereumTransactionHash *string `json:"ethereumTransactionHash"`
	HederaTransactionHash   string  `json:"hederaTransactionHash"`
}

type TransactionStatus struct {
	TransactionPayload
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

var (
	port                 int
	networkQueue         chan TransactionPayload
	mirrorQueue          chan TransactionPayload
	networkUrl           string
	networkAccount       int
	operatorAccount      int
	operatorAccountKey   string
	mirrorNodeUrl        string
	shadowingApiUrl      string
	fullLogFilePath      string
	logFile              *os.File
	networkWorkers       int
	mirrorWorkers        int
	networkQueueCapacity int
	mirrorQueueCapacity  int
	wg                   sync.WaitGroup

	successStatusCodes = map[int]struct{}{
		http.StatusOK:                   {},
		http.StatusCreated:              {},
		http.StatusAccepted:             {},
		http.StatusNonAuthoritativeInfo: {},
		http.StatusNoContent:            {},
		http.StatusResetContent:         {},
		http.StatusPartialContent:       {},
		http.StatusMultiStatus:          {},
		http.StatusAlreadyReported:      {},
		http.StatusIMUsed:               {},
	}
)

func main() {
	defaultNumWorkers := 128
	defaultChannelCapacity := 65536

	port = getEnvAsInt("PORT", 8081)
	networkWorkers = getEnvAsInt("NETWORK_WORKERS", defaultNumWorkers)
	mirrorWorkers = getEnvAsInt("MIRROR_WORKERS", defaultNumWorkers)
	networkQueueCapacity = getEnvAsInt("NETWORK_QUEUE_CAPACITY", defaultChannelCapacity)
	mirrorQueueCapacity = getEnvAsInt("MIRROR_QUEUE_CAPACITY", defaultChannelCapacity)
	logFilePath := getEnv("LOG_FILE_PATH", "logs/")
	logFileName := getEnv("LOG_FILE_NAME", "transactions.log")
	fullLogFilePath := filepath.Join(logFilePath, logFileName)

	networkUrl = getEnv("NETWORK_URL", "http://127.0.0.1:50211")
	networkAccount = getEnvAsInt("NETWORK_ACCOUNT", 3)
	operatorAccount = getEnvAsInt("OPERATOR_ACCOUNT", 2)
	operatorAccountKey = getEnv("OPERATOR_ACCOUNT_KEY", "302e020100300506032b65700422042091132178e72057a1d7528025956fe39b0b847f200ab59b2fdd367017f3087137")
	mirrorNodeUrl = getEnv("MIRROR_NODE_URL", "http://127.0.0.1:5551")
	shadowingApiUrl = getEnv("SHADOWING_API_URL", "http://127.0.0.1:3005")

	networkQueue = make(chan TransactionPayload, networkQueueCapacity)
	mirrorQueue = make(chan TransactionPayload, mirrorQueueCapacity)

	var err error
	logFile, err = os.OpenFile(fullLogFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer logFile.Close()

	operatorAccountID := hedera.AccountID{Account: uint64(operatorAccount)}
	privateKey, err := hedera.PrivateKeyFromString(operatorAccountKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	network := map[string]hedera.AccountID{
		networkUrl: {Account: uint64(networkAccount)},
	}
	nodeClient := hedera.ClientForNetwork(network)
	defer nodeClient.Close()
	nodeClient.SetOperator(operatorAccountID, privateKey)
	for i := 0; i < networkWorkers; i++ {
		wg.Add(1)
		go hederaWorker(i, networkQueue, nodeClient)
	}

	mirrorClient := &http.Client{
		Timeout: 10 * time.Second,
	}
	for i := 0; i < mirrorWorkers; i++ {
		wg.Add(1)
		go mirrorWorker(i, mirrorQueue, mirrorClient)
	}

	http.HandleFunc("/check-transaction", handleCheckTransaction)
	address := fmt.Sprintf(":%d", port)
	log.Printf("Listening on %s", address)
	log.Fatal(http.ListenAndServe(address, nil))

	wg.Wait()
}

func handleCheckTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var payload TransactionPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// timestamp, err := time.Parse(time.RFC3339, payload.Timestamp)
	// if err != nil {
	// 	http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
	// 	return
	// }
	// txTimestamp, err := time.Parse(time.RFC3339, payload.TxTimestamp)
	// if err != nil {
	// 	http.Error(w, "Invalid timestamp format", http.StatusBadRequest)
	// 	return
	// }

	//if tooLate(timestamp, txTimestamp) {
	//	mirrorQueue <- payload
	//	log.Printf("Transaction %s sent directly to mirror queue due to old timestamp.", payload.TxID)
	//} else {
	//	networkQueue <- payload
	//	log.Printf("Transaction %s sent to network node queue.", payload.TxID)
	//}

	mirrorQueue <- payload
	log.Printf("Transaction %s sent to mirror queue.", payload.TxID)
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte("OK"))
}

func hederaWorker(id int, payloads <-chan TransactionPayload, client *hedera.Client) {
	defer wg.Done()
	for payload := range payloads {
		log.Printf("Hedera Worker %d processing transaction %s", id, payload.TxID)
		status, err := getTransactionReceiptFromHederaNode(client, payload)
		if err != nil {
			mirrorQueue <- payload
			log.Printf("Hedera Worker %d failed to get receipt of transaction %s, sent to mirror queue. Error was: %v", id, payload.TxID, err)
		} else {
			log.Printf("Status of transaction %s is: %s", payload.TxID, status.String())
			sendAndLogToFile(payload, status.String(), nil)
		}
	}
}

func mirrorWorker(id int, payloads <-chan TransactionPayload, client *http.Client) {
	defer wg.Done()
	for payload := range payloads {
		log.Printf("Mirror Worker %d processing transaction %s", id, payload.TxID)
		time.Sleep(2 * time.Second)
		status, err := checkTransactionOnMirrorNode(client, payload)
		if err != nil {
			log.Printf("Mirror Worker %d failed to get the status of transaction %s from the mirror node: %v", id, payload.TxID, err)
			sendAndLogToFile(payload, "", fmt.Errorf("error getting status fgrom node, transaction failed or not executed (mirror node): %v", err))
		} else {
			log.Printf("Status of transaction %s is: %s", payload.TxID, status)
			sendAndLogToFile(payload, status, nil)
		}
	}
}

func getTransactionReceiptFromHederaNode(client *hedera.Client, payload TransactionPayload) (hedera.Status, error) {

	transactionId, err := hedera.TransactionIdFromString(payload.TxID)
	if err != nil {
		return hedera.StatusUnknown, err
	}

	receipt, err := hedera.NewTransactionReceiptQuery().
		SetTransactionID(transactionId).
		Execute(client)
	if err != nil {
		return hedera.StatusUnknown, err
	}
	log.Printf("Hedera node's receipt for transaction %s:\n%v", payload.TxID, receipt)

	return receipt.Status, nil
}

func checkTransactionOnMirrorNode(client *http.Client, payload TransactionPayload) (string, error) {
	type Transaction struct {
		Result        string `json:"result"`
		TransactionID string `json:"transaction_id"`
	}

	type Response struct {
		Transactions []Transaction `json:"transactions"`
	}

	transactionId := convertTransactionIdForMirrorNode(payload.TxID)

	url := mirrorNodeUrl + "/api/v1/transactions/" + transactionId
	log.Printf("checkTransactionOnMirrorNode: Sending GET request to: %s", url)
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		log.Println("OK")
	} else {
		return "", fmt.Errorf("received non-200 response: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response body: %v", err)
	}
	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		return "", err
	}

	var result string
	for _, tx := range response.Transactions {
		if tx.TransactionID == transactionId {
			result = tx.Result
			log.Printf("Transaction found in the response from Mirror Node:\n%s", result)
			break
		}
	}
	return result, nil
}

func sendAndLogToFile(payload TransactionPayload, status string, error error) {
	transactionStatus := TransactionStatus{
		TransactionPayload: payload,
		Status:             status,
	}
	if error != nil {
		transactionStatus.Error = error.Error()
	}
	jsonBytes, err := json.Marshal(transactionStatus)
	if err != nil {
		log.Printf("Error marshaling transaction %s to JSON: %v", payload.TxID, err)
		return
	}

	err = sendToShadowingApi(jsonBytes)
	if err != nil {
		log.Printf("Error sending the transaction: %v", err)
	}

	err = logToFile(jsonBytes)
	if err != nil {
		log.Printf("Failed to log transaction: %v", err)
	}
}

func sendToShadowingApi(jsonBytes []byte) error {
	url := shadowingApiUrl + "/contract-value"
	log.Printf("sendToShadowingApi: Sending POST request to: %s", url)
	log.Println("Request data:")
	logPrettyJSON(jsonBytes)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("error sending data to Shadowing API: %v", err)
	}
	defer resp.Body.Close()

	if _, ok := successStatusCodes[resp.StatusCode]; ok {
		log.Printf("OK (%d)", resp.StatusCode)
		return nil
	} else {
		return fmt.Errorf("error sending data to Shadowing API. Request failed with status code: %d", resp.StatusCode)
	}

}

func logToFile(jsonBytes []byte) error {
	logEntry := time.Now().Format(time.RFC3339) + " - " + string(jsonBytes) + "\n"
	var _, err = logFile.WriteString(logEntry)
	if err != nil {
		return fmt.Errorf("failed to write to log file: %v", err)
	}
	return nil
}

func logPrettyJSON(jsonData []byte) {
	var prettyJSON map[string]interface{}

	if err := json.Unmarshal(jsonData, &prettyJSON); err != nil {
		log.Printf("Error parsing JSON: %v", err)
		return
	}

	pretty, err := json.MarshalIndent(prettyJSON, "", "  ")
	if err != nil {
		log.Printf("Error formatting JSON: %v", err)
		return
	}

	log.Println(string(pretty))
}

func getEnv(key string, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func getEnvAsInt(key string, defaultValue int) int {
	value, err := strconv.Atoi(getEnv(key, ""))
	if err != nil {
		log.Printf("Invalid value for %s, using default: %v", key, defaultValue)
		return defaultValue
	}
	return value
}

func convertTransactionIdForMirrorNode(input string) string {
	re := regexp.MustCompile(`@([^.]*)\.`)
	result := re.ReplaceAllStringFunc(input, func(s string) string {
		return "-" + s[1:len(s)-1] + "-"
	})
	return result
}

func tooLate(timestamp time.Time, txTimestamp time.Time) bool {
	timeDifference := time.Since(timestamp)
	adjustedTxTimestamp := txTimestamp.Add(timeDifference)
	return adjustedTxTimestamp.Sub(txTimestamp) > 3*time.Minute
}
