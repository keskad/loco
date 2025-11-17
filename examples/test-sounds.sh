#!/bin/bash
LOCO=5   # Loco number

./bin/loco fn set --loco $LOCO 0  # Pc0
./bin/loco fn set --loco $LOCO 1  # Engine sound
./bin/loco fn set --loco $LOCO 7  # F7
./bin/loco fn set --loco $LOCO 16 # F16
sleep 10

echo "Accelerating... ;)"
./bin/loco speed --loco $LOCO 5
sleep 2
./bin/loco speed --loco $LOCO 12 
sleep 2
./bin/loco speed --loco $LOCO 30 
./bin/loco fn set --loco $LOCO 3 # F3 Horn
sleep 0.5
./bin/loco fn set --loco $LOCO 3 --off # F3 Horn off after 0.5s
sleep 5
./bin/loco speed --loco $LOCO 60 
sleep 4
./bin/loco speed --loco $LOCO 80 
sleep 8
./bin/loco speed --loco $LOCO 100 
sleep 5
./bin/loco speed --loco $LOCO 120 
sleep 5
echo "Applying brakes..."
./bin/loco speed --loco $LOCO 80
sleep 5
./bin/loco speed --loco $LOCO 50
sleep 8
./bin/loco speed --loco $LOCO 20
sleep 8
./bin/loco speed --loco $LOCO 10
sleep 8
./bin/loco speed --loco $LOCO 5
sleep 5
./bin/loco speed --loco $LOCO 0
sleep 10
echo "Shutting down..."
./bin/loco fn set --loco $LOCO 1 --off  # Turn off Engine sound
./bin/loco fn set --loco $LOCO 0 --off  # Turn off Pc0
./bin/loco fn set --loco $LOCO 7 --off  # Turn off F7
./bin/loco fn set --loco $LOCO 16 --off # Turn off F16
echo "Done!"
