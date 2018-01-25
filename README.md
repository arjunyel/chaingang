# chaingang

## Example Usage

Install [Go Dep](https://github.com/golang/dep)

Create an "env.list" file with the Bittrex keys:

```bash
BITTREXKEY=
BITTREXSECRET=
```

Verify dependencies

```bash
dep ensure -update
```

Run app

```bash
docker run --env-file ./env.list chaingang:latest
```