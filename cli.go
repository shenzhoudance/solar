package solar

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/pkg/errors"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var (
	app       = kingpin.New("solar", "Solidity smart contract deployment management.")
	solarRPC  = app.Flag("rpc", "RPC provider url").Envar("QTUM_RPC").String()
	solarEnv  = app.Flag("env", "Environment name").Envar("SOLAR_ENV").Default("development").String()
	solarRepo = app.Flag("repo", "Path of contracts repository").Envar("SOLAR_REPO").String()
	appTasks  = map[string]func() error{}

	solcOptimize   = app.Flag("optimize", "[solc] should Enable bytecode optimizer").Default("true").Bool()
	solcAllowPaths = app.Flag("allow-paths", "[solc] Allow a given path for imports. A list of paths can be supplied by separating them with a comma.").Default("").String()
)

type solarCLI struct {
	rpc     *qtumRPC
	rpcOnce sync.Once

	repo     *contractsRepository
	repoOnce sync.Once

	reporter     *events
	reporterOnce sync.Once
}

var solar = &solarCLI{
// reporter: &events{
// 	in: make(chan interface{}),
// }
}

func (c *solarCLI) Reporter() *events {
	c.reporterOnce.Do(func() {
		c.reporter = &events{
			in: make(chan interface{}),
		}

		go c.reporter.Start()
	})

	return c.reporter
}

func (c *solarCLI) SolcOptions() (*CompilerOptions, error) {
	allowPathsStr := *solcAllowPaths
	if allowPathsStr == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, errors.Wrap(err, "solc options")
		}

		allowPathsStr = cwd
	}

	allowPaths := strings.Split(allowPathsStr, ",")

	return &CompilerOptions{
		NoOptimize: !*solcOptimize,
		AllowPaths: allowPaths,
	}, nil
}

func (c *solarCLI) RPC() *qtumRPC {
	log := log.New(os.Stderr, "", log.Lshortfile)
	c.rpcOnce.Do(func() {
		rawurl := *solarRPC

		if rawurl == "" {
			log.Fatalln("Please specify RPC url by setting QTUM_RPC or using the --rpc flag")
		}

		rpcURL, err := url.ParseRequestURI(rawurl)

		if err != nil {
			log.Fatalf("Invalid RPC url: %#v", rawurl)
		}

		c.rpc = &qtumRPC{rpcURL}
	})

	return c.rpc
}

// Open the file `solar.{SOLAR_ENV}.json` as contracts repository
func (c *solarCLI) ContractsRepository() *contractsRepository {
	c.repoOnce.Do(func() {
		var repoFilePath string
		if *solarRepo != "" {
			repoFilePath = *solarRepo
		} else {
			repoFilePath = fmt.Sprintf("solar.%s.json", *solarEnv)
		}

		repo, err := openContractsRepository(repoFilePath)
		if err != nil {
			fmt.Println("Cannot open contracts repo:", repoFilePath)
			os.Exit(1)
		}

		c.repo = repo
	})

	return c.repo
}

func (c *solarCLI) Deployer() *Deployer {
	return &Deployer{
		rpc:  c.RPC(),
		repo: c.ContractsRepository(),
	}
}

func Main() {
	cmdName, err := app.Parse(os.Args[1:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	task := appTasks[cmdName]
	err = task()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
