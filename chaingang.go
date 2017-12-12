package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/toorop/go-bittrex"
)

var parentCoins = map[string]*parentCoin{}
var childCoins = map[string]*childCoin{}

func updateMarketSummaries(bittrexClient *bittrex.Bittrex) ([]bittrex.MarketSummary, error) {
	marketSummaries, err := bittrexClient.GetMarketSummaries()
	return marketSummaries, err
}

func populateCoins(marketSummaries []bittrex.MarketSummary) {
	for _, marketSummary := range marketSummaries {

		newParentCoinName := strings.Split(marketSummary.MarketName, "-")[0]
		newChildCoinName := strings.Split(marketSummary.MarketName, "-")[1]
		//calculate nonUSDT
		if _, childIsParent := parentCoins[newChildCoinName]; childIsParent {
			switch newParentCoinName {
			case "BTC":
				switch newChildCoinName {
				case "ETH":
					parentCoins[newChildCoinName].Btc = marketSummary.Last
					parentCoins[newParentCoinName].Eth = decimal.NewFromFloat(1).Div(marketSummary.Last)
				case "USDT":
					parentCoins[newChildCoinName].Btc = marketSummary.Last
					parentCoins[newParentCoinName].Usdt = decimal.NewFromFloat(1).Div(marketSummary.Last)
				}
			case "ETH":
				switch newChildCoinName {
				case "BTC":
					parentCoins[newChildCoinName].Eth = marketSummary.Last
					parentCoins[newParentCoinName].Btc = decimal.NewFromFloat(1).Div(marketSummary.Last)
				case "USDT":
					parentCoins[newChildCoinName].Eth = marketSummary.Last
					parentCoins[newParentCoinName].Usdt = decimal.NewFromFloat(1).Div(marketSummary.Last)
				}
			case "USDT":
				switch newChildCoinName {
				case "BTC":
					parentCoins[newChildCoinName].Usdt = marketSummary.Last
					parentCoins[newParentCoinName].Btc = decimal.NewFromFloat(1).Div(marketSummary.Last)
				case "ETH":
					parentCoins[newChildCoinName].Usdt = marketSummary.Last
					parentCoins[newParentCoinName].Eth = decimal.NewFromFloat(1).Div(marketSummary.Last)
				}
			default:
				fmt.Printf("Warning : No support for parent coin : %v", newParentCoinName)
			}
		}

		if _, newChildCoinExists := childCoins[newChildCoinName]; !newChildCoinExists {
			childCoins[newChildCoinName] = &childCoin{
				Name:        newChildCoinName,
				ParentCoins: make([]string, 0),
			}
		}
		switch newParentCoinName {
		case "BTC":
			childCoins[newChildCoinName].Btc = marketSummary.Last
		case "ETH":
			childCoins[newChildCoinName].Eth = marketSummary.Last
		default:
			fmt.Printf("Warning : No support for parent coin : %v", newParentCoinName)
		}
		childCoins[newChildCoinName].ParentCoins = append(childCoins[newChildCoinName].ParentCoins, newParentCoinName)
		parentCoins[newParentCoinName].ChildCoins = append(parentCoins[newParentCoinName].ChildCoins, newChildCoinName)

	}
}

func printCoinValues() {
	for childCoinName, childCoinValue := range childCoins {
		_, childIsParent := parentCoins[childCoinName]
		if len(childCoinValue.ParentCoins) == 2 && !childIsParent {
			fmt.Printf("%v:\n", childCoinName)
			fmt.Printf("\tBTC : %v\n", childCoinValue.Btc)
			fmt.Printf("\tETH : %v\n", childCoinValue.Eth)
			fmt.Println()
		}

	}
	fmt.Println("-------------------------------------------------------------")
	for parentCoinName, parentCoinValue := range parentCoins {
		fmt.Printf("%v:\n", parentCoinName)
		fmt.Printf("\tBTC : %v\n", parentCoinValue.Btc)
		fmt.Printf("\tETH : %v\n", parentCoinValue.Eth)
		fmt.Printf("\tUSDT: %v\n", parentCoinValue.Usdt)

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

		//populateParentCoin
		for _, parentCoinName := range parentCoinNames {

			parentCoins[parentCoinName] = &parentCoin{
				Name: parentCoinName,
			}
		}

		for {
			marketSummaries, err := updateMarketSummaries(bittrexClient)
			go func() {
				populateCoins(marketSummaries)
				//calculateCoinValues()
				printCoinValues()
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
	Btc        decimal.Decimal
	Eth        decimal.Decimal
	Usdt       decimal.Decimal
	ChildCoins []string
}

type childCoin struct {
	Name        string
	Btc         decimal.Decimal
	Eth         decimal.Decimal
	ParentCoins []string
}
