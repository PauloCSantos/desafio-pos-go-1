package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Cotacao struct {
	Bid float64 `json:"bid"`
}

type FullCotacao struct {
	USDBRL map[string]interface{} `json:"USDBRL"`
}

func main() {
	/*
		1º Fazer uma req https://economia.awesomeapi.com.br/json/last/USD-BRL, timeout 200ms
		2º Persistir o resultado no SQLite, timeout 10ms
		3º Retornar um JSON com apenas o campo "bid"
		4º Context deve gerar log em caso de timeout
	*/
	// Variaveis de config
	apiPath := "https://economia.awesomeapi.com.br/json/last/USD-BRL"
	apiTimeout := 200 * time.Millisecond
	dbTimeout := 10 * time.Millisecond
	port := "8080"
	// Conexao com o bd
	db, err := sql.Open("sqlite3", ":memory:")

	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	// Cria a tabela caso nao existir
	err = createTable(db)
	if err != nil {
		log.Fatal(err)
	}
	// Config server
	mux := http.NewServeMux()
	mux.HandleFunc("/cotacao", func(w http.ResponseWriter, r *http.Request) {
		handler(w, db, apiPath, apiTimeout, dbTimeout)
	})
	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}
	// Server init
	log.Printf("Servidor escutando na porta %s", port)
	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, db *sql.DB, apiPath string, apiTimeout, dbTimeout time.Duration) {
	// Dados da API, em json
	fullData, err := getFullData(apiPath, apiTimeout)
	if err != nil {
		log.Printf("Erro buscando os dados: %v", err)
		http.Error(w, "Erro interno do servidor", http.StatusInternalServerError)
		return
	}
	// Persiste os dados no banco
	err = saveFullData(db, dbTimeout, fullData, false) // Para confirmar que os dados foram inseridos use "true"
	if err != nil {
		log.Printf("Erro persistindo os dados: %v", err)
		http.Error(w, "Erro interno do servidor", http.StatusInternalServerError)
		return
	}

	bidStr, ok := fullData.USDBRL["bid"].(string)
	if !ok {
		log.Printf("Campo bid nao encontrado")
		http.Error(w, "Erro interno do servidor", http.StatusInternalServerError)
		return
	}

	bid, err := strconv.ParseFloat(bidStr, 64)
	if err != nil {
		log.Printf("Erro na conversao do bid: %v", err)
		http.Error(w, "Erro interno do servidor", http.StatusInternalServerError)
		return
	}
	response := Cotacao{Bid: bid}

	jsonData, err := json.Marshal(response)
	if err != nil {
		log.Printf("Erro na conversao para JSON: %v", err)
		http.Error(w, "Erro interno do servidor", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)
}

func getFullData(url string, timeout time.Duration) (FullCotacao, error) {
	// Contexto com timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Config da requisicao com timeout
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return FullCotacao{}, err
	}
	// Realizando a requisicao
	resp, err := client.Do(req)
	if err != nil {
		return FullCotacao{}, err
	}
	defer resp.Body.Close()
	// Lendo o retorno
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return FullCotacao{}, err
	}
	// Convertendo o resultado da req
	var data FullCotacao
	err = json.Unmarshal(body, &data)
	if err != nil {
		return FullCotacao{}, err
	}

	return data, nil
}

func saveFullData(db *sql.DB, timeout time.Duration, data FullCotacao, confirmaInsert bool) error {
	// Contexto com timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	// Recuperando os dados
	code, _ := data.USDBRL["code"].(string)
	codein, _ := data.USDBRL["codein"].(string)
	name, _ := data.USDBRL["name"].(string)
	highStr, _ := data.USDBRL["high"].(string)
	lowStr, _ := data.USDBRL["low"].(string)
	varBidStr, _ := data.USDBRL["varBid"].(string)
	pctChangeStr, _ := data.USDBRL["pctChange"].(string)
	bidStr, _ := data.USDBRL["bid"].(string)
	askStr, _ := data.USDBRL["ask"].(string)
	timestampStr, _ := data.USDBRL["timestamp"].(string)
	createDate, _ := data.USDBRL["create_date"].(string)
	// Convertendos para o tipo correto
	high, _ := strconv.ParseFloat(highStr, 64)
	low, _ := strconv.ParseFloat(lowStr, 64)
	varBid, _ := strconv.ParseFloat(varBidStr, 64)
	pctChange, _ := strconv.ParseFloat(pctChangeStr, 64)
	bid, _ := strconv.ParseFloat(bidStr, 64)
	ask, _ := strconv.ParseFloat(askStr, 64)
	timestamp, _ := strconv.ParseInt(timestampStr, 10, 64)
	// Preparando o insert
	stmt, err := db.PrepareContext(ctx, `
		INSERT INTO cotacoes (code, codein, name, high, low, varBid, pctChange, bid, ask, timestamp, create_date)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	// Inserindo os dados
	_, err = stmt.ExecContext(ctx, code, codein, name, high, low, varBid, pctChange, bid, ask, timestamp, createDate)
	if err != nil {
		return err
	}
	if confirmaInsert {
		// Recupera os registros para confirmacao
		rows, err := db.QueryContext(ctx, `SELECT * FROM cotacoes`)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var code, codein, name, createDate string
			var high, low, varBid, pctChange, bid, ask float64
			var timestamp int64

			err = rows.Scan(&code, &codein, &name, &high, &low, &varBid, &pctChange, &bid, &ask, &timestamp, &createDate)
			if err != nil {
				return err
			}

			fmt.Printf("code: %s, codein: %s, name: %s, high: %f, low: %f, varBid: %f, pctChange: %f, bid: %f, ask: %f, timestamp: %d, create_date: %s\n",
				code, codein, name, high, low, varBid, pctChange, bid, ask, timestamp, createDate)
		}

		if err = rows.Err(); err != nil {
			return err
		}
	}
	return nil
}

func createTable(db *sql.DB) error {
	// Cria a tabela caso nao exista
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS cotacoes (
			code TEXT NOT NULL,
			codein TEXT NOT NULL,
			name TEXT NOT NULL,
			high REAL NOT NULL,
			low REAL NOT NULL,
			varBid REAL NOT NULL,
			pctChange REAL NOT NULL,
			bid REAL NOT NULL,
			ask REAL NOT NULL,
			timestamp INTEGER NOT NULL,
			create_date DATETIME NOT NULL
		);
    `)
	if err != nil {
		return err
	}

	return nil
}
