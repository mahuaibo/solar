package solar

import (
	"fmt"
	"strings"
	"time"

	"github.com/kr/pretty"
	"github.com/pkg/errors"
)

func init() {
	cmd := app.Command("deploy", "Compile Solidity contracts.")

	force := cmd.Flag("force", "Overwrite previously deployed contract with the same name").Bool()
	noconfirm := cmd.Flag("no-confirm", "Don't wait for network to confirm deploy").Bool()

	targets := cmd.Arg("targets", "Solidity contracts to deploy.").Strings()

	appTasks["deploy"] = func() (err error) {
		// verify before deploy

		targets := *targets

		if len(targets) == 0 {
			return errors.New("nothing to deploy")
		}

		deployer := solar.Deployer()

		for _, target := range targets {
			dt := parseDeployTarget(target)

			pretty.Printf("Deploying %s as %s\n", dt.FilePath, dt.Name)
			err := deployer.CreateContract(dt.Name, dt.FilePath, *force)
			if err != nil {
				fmt.Println(err)
			}
			// TODO: verify no duplicate target names
			// TODO: verify all contracts before deploy
		}

		if *noconfirm == false {
			repo := solar.ContractsRepository()
			err := repo.ConfirmAll()
			if err != nil {
				return err
			}

			fmt.Println("All deployed contracts confirmed")
		}

		return
	}
}

type deployTarget struct {
	Name     string
	FilePath string
}

func parseDeployTarget(target string) deployTarget {
	parts := strings.Split(target, ":")

	filepath := parts[0]

	var name string
	if len(parts) == 2 {
		name = parts[1]
	} else {
		name = stringLowerFirstRune(basenameNoExt(filepath))
	}

	// TODO verify name for valid JS name

	t := deployTarget{
		Name:     name,
		FilePath: filepath,
	}

	return t
}

type Deployer struct {
	rpc  *qtumRPC
	repo *contractsRepository
}

func (d *Deployer) CreateContract(name, filepath string, overwrite bool) (err error) {
	if !overwrite && d.repo.Exists(name) {
		return errors.Errorf("Contract name alredy used: %s", name)
	}

	gasLimit := 300000

	rpc := d.rpc

	contract, err := compileSource(filepath, CompilerOptions{})
	if err != nil {
		return errors.Wrap(err, "compile")
	}

	var tx TransactionReceipt

	err = rpc.Call(&tx, "createcontract", contract.Bin.String(), gasLimit)

	if err != nil {
		return errors.Wrap(err, "createcontract")
	}

	// fmt.Println("tx", tx.Address)
	// fmt.Println("contract name", contract.Name)

	deployedContract := &DeployedContract{
		Name:             contract.Name,
		CompiledContract: *contract,
		TransactionID:    tx.TxID,
		Address:          tx.Address,
		CreatedAt:        time.Now(),
	}

	err = d.repo.Set(name, deployedContract)
	if err != nil {
		return
	}

	err = d.repo.Commit()
	if err != nil {
		return
	}
	// pretty.Println("rpc err", string(res.RawError))

	return nil
}
