package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/flipped-aurora/gin-vue-admin/server/internal/testmysql"
	"github.com/flipped-aurora/gin-vue-admin/server/service/tenant"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 0 || (args[0] != "provision" && args[0] != "seed-demo") {
		return errors.New("usage: dealflow-tenant provision -code CODE -name NAME -admin ID | seed-demo -admin ID -sales ID -manager ID -finance ID")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	admin := flags.Uint64("admin", 0, "existing GVA administrator user ID")
	var provision tenant.ProvisionInput
	var demo tenant.DemoUsers
	if args[0] == "provision" {
		flags.StringVar(&provision.Code, "code", "", "tenant code")
		flags.StringVar(&provision.Name, "name", "", "tenant display name")
	} else {
		flags.Uint64Var(&demo.Sales, "sales", 0, "existing GVA sales user ID")
		flags.Uint64Var(&demo.Manager, "manager", 0, "existing GVA manager user ID")
		flags.Uint64Var(&demo.Finance, "finance", 0, "existing GVA finance user ID")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected tenant command arguments")
	}
	cfg, err := testmysql.Config(os.Getenv("DEALFLOW_TEST_DSN"))
	if err != nil {
		return err
	}
	db, err := gorm.Open(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("open tenant test database: %w", err)
	}
	pool, err := db.DB()
	if err != nil {
		return err
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	service := tenant.New(db)
	if args[0] == "seed-demo" {
		demo.Admin = *admin
		if err := service.SeedDemo(ctx, demo); err != nil {
			return err
		}
		fmt.Println("Demo tenants verified: demo-a, demo-b")
		return nil
	}

	provision.AdminUserID = *admin
	created, err := service.Provision(ctx, provision)
	if err != nil {
		return err
	}
	fmt.Printf("Provisioned tenant %s (id=%d)\n", created.Code, created.ID)
	return nil
}
