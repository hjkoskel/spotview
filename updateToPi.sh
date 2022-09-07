#On my workflow I compile on laptop and copy result to pi for testing. change ip
GOARCH=arm GOOS=linux go build && scp spotview pi@192.168.1.112:/home/pi/
