package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	apiv1 "github.com/khanghh/vscode-server/internal/api/v1"
	"github.com/khanghh/vscode-server/internal/core"
	"github.com/urfave/cli/v2"
)

var (
	app       *cli.App
	gitCommit string
	gitDate   string
	gitTag    string
)

var (
	listenFlag = &cli.StringFlag{
		Name:  "listen",
		Usage: "Address to listen on",
		Value: ":3000",
	}
	rootDirFlag = &cli.StringFlag{
		Name:  "rootdir",
		Usage: "Directory to serve files to the web IDE",
		Value: "/tmp",
	}
	webDirFlag = &cli.StringFlag{
		Name:  "webdir",
		Usage: "Directory to serve web static files",
		Value: "./dist",
	}
	debugFlag = &cli.BoolFlag{
		Name:  "debug",
		Usage: "Enable debug logging",
	}
)

func init() {
	app = cli.NewApp()
	app.EnableBashCompletion = true
	app.Usage = ""
	app.Flags = []cli.Flag{
		debugFlag,
		rootDirFlag,
		webDirFlag,
		listenFlag,
	}
	app.Commands = []*cli.Command{
		{
			Name:   "version",
			Action: printVersion,
		},
	}
	app.Action = run
}

func printVersion(cli *cli.Context) error {
	fmt.Println(cli.App.Name)
	fmt.Printf(" Version:\t%s\n", gitTag)
	fmt.Printf(" Commit:\t%s\n", gitCommit)
	fmt.Printf(" Built Time:\t%s\n", gitDate)
	return nil
}

func mustInitLogger(debug bool) {
	logLevel := slog.LevelInfo
	if debug {
		logLevel = slog.LevelDebug
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
}

func run(cli *cli.Context) error {
	mustInitLogger(cli.Bool(debugFlag.Name))
	listenAddr := cli.String(listenFlag.Name)

	webDir := cli.String(webDirFlag.Name)
	if _, err := os.Stat(webDir); os.IsNotExist(err) {
		log.Fatalf("web directory \"%s\" does not exist", webDir)
	}

	rootDir := cli.String(rootDirFlag.Name)
	if rootDir == "" {
		log.Fatal("must provide work directory")
	}

	lfs := core.NewLocalFileService(rootDir)

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${ip} | ${method} | ${path} ${queryParams} | ${error}\n",
		TimeFormat: "2006-01-02 15:04:05",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH,OPTIONS",
		AllowHeaders: "*",
	}))

	// Serve the built VS Code Web frontend from webDir at "/"
	app.Static("/", webDir)
	// Setup API routes at "/api/v1"
	if err := apiv1.SetupRoutes(app, lfs); err != nil {
		log.Fatal(err)
	}

	return app.Listen(listenAddr)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
