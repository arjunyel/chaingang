package main

import (
	"fmt"
	"os"
	"github.com/toorop/go-bittrex"
)

func getBittrexMarketRecords(bittrexClient *bittrex.Bittrex, markets []bittrex.Market) []marketRecord{
	marketRecords := []marketRecord{}
	for _, market := range markets {
		if(market.IsActive){
			ticker, err := bittrexClient.GetTicker(market.MarketName);
			if(err == nil){
				record := marketRecord {
					Market: market,
					Ticker: ticker,
				}
				marketRecords = append(marketRecords, record)
			} else {
				fmt.Println(err)
			}
		}
	}
	return marketRecords
}

func main() {
	var bittrexKey, coinbaseKey, bittrexSecret string
	fmt.Printf("chaingang running\n")
		for i := 1; i < len(os.Args); i += 2 {
			switch os.Args[i]{
				case "-b":
					bittrexKey = os.Args[i + 1]
				case "-c":
					coinbaseKey = os.Args[i + 1]
			  case "-s":
					bittrexSecret = os.Args[i + 1]
				default:
					panic("unrecognized argument")
			}
		}

	fmt.Printf("\tbittrexKey: %v\n", bittrexKey)
	fmt.Printf("\tbittrexSecret: %v\n", bittrexSecret)
	fmt.Printf("\tcoinbaseKey: %v\n", coinbaseKey)

	if(bittrexKey != "" && bittrexSecret != ""){
		bittrexClient := bittrex.New(bittrexKey, bittrexSecret)
		markets, err := bittrexClient.GetMarkets()
		if(err == nil){
			//this is where we start looping updates
			//might have to work with 'SubscribeExchangeUpdate'
			marketRecords := getBittrexMarketRecords(bittrexClient, markets)
			for _,record := range marketRecords {
				fmt.Printf("%v\n", record.Market.MarketName)
				fmt.Printf("\t %v\n", record.Ticker)
			}
		} else {
			fmt.Println(err)
		}

	} else {
		fmt.Println("please provide bittrex key and secret")
	}
}

type marketRecord struct {
	Market bittrex.Market
	Ticker bittrex.Ticker
}
