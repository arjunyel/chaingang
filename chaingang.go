package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/toorop/go-bittrex"
)

func updateMarketSummaries(bittrexClient *bittrex.Bittrex) ([]bittrex.MarketSummary, error) {
	marketSummaries, err := bittrexClient.GetMarketSummaries()
	return marketSummaries, err
}

func processMarketSummaries(parentCoins map[string]parentCoin, marketSummaries []bittrex.MarketSummary) {
	for _, marketSummary := range marketSummaries {

		newChildCoin := childCoin{
			Name:          strings.Split(marketSummary.MarketName, "-")[1],
			MarketSummary: marketSummary,
		}

		parentCoins[strings.Split(marketSummary.MarketName, "-")[0]].ChildCoins[newChildCoin.Name] = newChildCoin

	}
}

func main() {
	var parentCoinNames = [...]string{"BTC", "ETH", "USDT"}
	bittrexThreshhold := time.Duration(10) * time.Second
	var bittrexKey, coinbaseKey, bittrexSecret string
	fmt.Printf("chaingang running\n")
	for i := 1; i < len(os.Args); i += 2 {
		switch os.Args[i] {
		case "-b":
			bittrexKey = os.Args[i+1]
		case "-c":
			coinbaseKey = os.Args[i+1]
		case "-s":
			bittrexSecret = os.Args[i+1]
		default:
			panic("unrecognized argument")
		}
	}

	fmt.Printf("\tbittrexKey: %v\n", bittrexKey)
	fmt.Printf("\tbittrexSecret: %v\n", bittrexSecret)
	fmt.Printf("\tcoinbaseKey: %v\n", coinbaseKey)

	if bittrexKey != "" && bittrexSecret != "" {
		bittrexClient := bittrex.New(bittrexKey, bittrexSecret)
		parentCoins := make(map[string]parentCoin)

		//populateParentCoin
		for _, coinName := range parentCoinNames {
			parentCoinName := strings.Split(coinName, "-")[0]
			var UsdValue decimal.Decimal
			switch parentCoinName {
			case "BTC":
				UsdValue = decimal.NewFromFloat(11538.90)
			case "ETH":
				UsdValue = decimal.NewFromFloat(464.54)
			case "USDT":
				UsdValue = decimal.NewFromFloat(.99)
			}
			parentCoins[coinName] = parentCoin{
				Name:       coinName,
				USD:        UsdValue,
				ChildCoins: make(map[string]childCoin),
			}
		}

		for {
			marketSummaries, err := updateMarketSummaries(bittrexClient)
			go func() {
				processMarketSummaries(parentCoins, marketSummaries)

				for _, pCoin := range parentCoins {
					fmt.Println(pCoin.Name)
					for _, cCoin := range pCoin.ChildCoins {
						fmt.Printf("\t%v : %v\n", cCoin.Name, cCoin.MarketSummary.Last.Mul(pCoin.USD))
					}
				}

			}()
			if err == nil {

			} else {
				fmt.Println(err)
			}
			time.Sleep(bittrexThreshhold)
		}

	} else {
		fmt.Println("please provide bittrex key and secret")
	}
}

type marketRecord struct {
	MarketSummary bittrex.MarketSummary
}

type parentCoin struct {
	Name       string
	USD        decimal.Decimal
	ChildCoins map[string]childCoin
}

type childCoin struct {
	Name          string
	MarketSummary bittrex.MarketSummary
}
