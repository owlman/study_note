# Go Telemetry View

Telemetry data it is stored in files on the user machine. Users can run the
command `gotelemetry view` to view the data in a browser. The HTML page served
by the command will generate graphs based on the local copies of report uploads
and active counter files.

## Development

The static files are generated with a generator command. You can edit the source
files and run go generate to rebuild them.

    go generate ./content

Running the server with the `--dev` flag will watch and rebuild the static files
on save.

    go run ./cmd/gotelemetry view --dev
