package main

import (
	"fmt"
	"os"
	"github.com/toorop/go-bittrex"
)

func main() {
	var bittrexKey, coinbaseKey, bittrexSecret string
	const maxArguments = 3
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
		bittrex := bittrex.New(bittrexKey, bittrexSecret)
		markets, err := bittrex.GetMarkets()
		fmt.Println(err, markets)
	} else {
		fmt.Printf("please provide bittrex key and secret")
	}
}
