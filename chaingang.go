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

func populateCoins(parentCoins map[string]*parentCoin, childCoins map[string]*childCoin, marketSummaries []bittrex.MarketSummary) {
	for _, marketSummary := range marketSummaries {

		newParentCoinName := strings.Split(marketSummary.MarketName, "-")[0]
		newChildCoinName := strings.Split(marketSummary.MarketName, "-")[1]
		//calculate nonUSDT
		if _, childIsParent := parentCoins[newChildCoinName]; childIsParent {
			switch newParentCoinName {
			case "BTC":
				parentCoins[newChildCoinName].BTC = marketSummary.Last
			case "ETH":
				parentCoins[newChildCoinName].ETH = marketSummary.Last
			case "USDT":
				parentCoins[newChildCoinName].USDT = marketSummary.Last
			default:
				fmt.Printf("Warning : No support for parent coin : %v", newParentCoinName)
			}
		}

		if _, newChildCoinExists := childCoins[newChildCoinName]; !newChildCoinExists {
			childCoins[newChildCoinName] = &childCoin{
				Name:          newChildCoinName,
				MarketSummary: marketSummary,
				ParentCoins:   make([]string, 0),
			}
		}
		switch newParentCoinName {
		case "BTC":
			childCoins[newChildCoinName].BTC = marketSummary.Last
		case "ETH":
			childCoins[newChildCoinName].ETH = marketSummary.Last
		case "USDT":
			childCoins[newChildCoinName].USDT = marketSummary.Last
		default:
			fmt.Printf("Warning : No support for parent coin : %v", newParentCoinName)
		}
		childCoins[newChildCoinName].ParentCoins = append(childCoins[newChildCoinName].ParentCoins, newParentCoinName)
		parentCoins[newParentCoinName].ChildCoins = append(parentCoins[newParentCoinName].ChildCoins, newChildCoinName)

	}

}

func calculateCoinValues(parentCoins map[string]*parentCoin, childCoins map[string]*childCoin) {
	for parentCoinName, parentCoinValue := range parentCoins {

		for _, childCoinName := range parentCoinValue.ChildCoins {
			switch parentCoinName {
			case "BTC":
				childCoins[childCoinName].BTC = parentCoinValue.USDT.Mul(childCoins[childCoinName].BTC)
			case "ETH":
				childCoins[childCoinName].ETH = parentCoinValue.USDT.Mul(childCoins[childCoinName].ETH)
			}
		}
	}
}

func printCoinValues(parentCoins map[string]*parentCoin, childCoins map[string]*childCoin) {
	for childCoinName, childCoinValue := range childCoins {
		_, childIsParent := parentCoins[childCoinName]
		if len(childCoinValue.ParentCoins) == 2 && !childIsParent {
			fmt.Printf("\t\t%v:\n", childCoinName)
			fmt.Printf("\t\t\tBTC : %v\n", childCoinValue.BTC)
			fmt.Printf("\t\t\tETH : %v\n", childCoinValue.ETH)
			fmt.Println()
			if childCoinValue.BTC.GreaterThan(childCoinValue.ETH) {
				fmt.Printf("\t\t\tPER TRAN: $%v\n", childCoinValue.BTC.Add(childCoinValue.ETH.Neg()))
				fmt.Printf("\t\t\tBUY ETH : +%v%%\n", childCoinValue.BTC.Div(childCoinValue.ETH).Add(decimal.NewFromFloat(1).Neg()).Round(2).Mul(decimal.NewFromFloat(100)))
			} else if childCoinValue.ETH.GreaterThan(childCoinValue.BTC) {
				fmt.Printf("\t\t\tPER TRAN: $%v\n", childCoinValue.ETH.Add(childCoinValue.BTC.Neg()))
				fmt.Printf("\t\t\tBUY BTC : +%v%%\n", childCoinValue.ETH.Div(childCoinValue.BTC).Add(decimal.NewFromFloat(1).Neg()).Round(2).Mul(decimal.NewFromFloat(100)))
			}
		}

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
		parentCoins := make(map[string]*parentCoin)
		childCoins := make(map[string]*childCoin)

		//populateParentCoin
		for _, parentCoinName := range parentCoinNames {

			parentCoins[parentCoinName] = &parentCoin{
				Name: parentCoinName,
			}
		}

		for {
			marketSummaries, err := updateMarketSummaries(bittrexClient)
			go func() {
				populateCoins(parentCoins, childCoins, marketSummaries)
				calculateCoinValues(parentCoins, childCoins)
				printCoinValues(parentCoins, childCoins)
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
	BTC        decimal.Decimal
	ETH        decimal.Decimal
	USDT       decimal.Decimal
	ChildCoins []string
}

type childCoin struct {
	Name          string
	MarketSummary bittrex.MarketSummary
	BTC           decimal.Decimal
	ETH           decimal.Decimal
	USDT          decimal.Decimal
	ParentCoins   []string
}
