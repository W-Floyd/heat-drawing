#!/bin/bash

go build && ./heat-drawing -file image.png -image -angle 45 -width 5 -height 5 -density 1.6 -force-dimensions

exit
