package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	"log"
	"math"
	"os"

	"image/color"
	_ "image/jpeg"
	_ "image/png"

	"github.com/tdewolff/canvas"
	"github.com/tdewolff/canvas/renderers"
	"golang.org/x/image/colornames"
)

type Vec2 [2]float64

type scaling Vec2

type rectangle Vec2

type point Vec2

type pixel Vec2

type path []point

type configuration struct {
	size                      rectangle
	position                  point
	forceDimensions           bool
	lineSeparation, nozzleGap float64
	plotAngle, plotDirection  float64
	plotDensity               float64
	speedBlack, speedWhite    float64
	speedCoefficient          float64
	plotStart, plotEnd        string
	drawImage                 bool
}

var (
	imageScale scaling
	config     configuration
	sourceSize rectangle
	defaults   = configuration{
		size: rectangle{
			100, 100,
		},
		position: point{
			0, 0,
		},
		forceDimensions:  false,
		lineSeparation:   0.4,
		nozzleGap:        0.2,
		plotAngle:        45,
		plotDirection:    45,
		plotDensity:      0.5,
		speedBlack:       3,
		speedWhite:       10,
		speedCoefficient: 1,
		plotStart:        "STARTPLOT",
		plotEnd:          "ENDPLOT",

		drawImage: false,
	}
)

func errorFail(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func scale(source, target rectangle, forceSize bool) scaling {

	widthScale := target[0] / source[0]
	heightScale := target[1] / source[1]

	if forceSize {
		return scaling{widthScale, heightScale}
	} else if source == target {
		return scaling{1, 1}
	} else if widthScale < heightScale {
		return scaling{widthScale, widthScale}
	}
	return scaling{heightScale, heightScale}

}

func main() {

	var imageFilename string

	flag.StringVar(&imageFilename, "file", "", "Image file to process")

	flag.Float64Var(&config.size[0], "width", defaults.size[0], "Maximum image width (in mm)")
	flag.Float64Var(&config.size[1], "height", defaults.size[1], "Maximum image height (in mm)")

	flag.Float64Var(&config.position[0], "start-x", defaults.position[0], "Maximum image width (in mm)")
	flag.Float64Var(&config.position[1], "start-y", defaults.position[1], "Maximum image height (in mm)")

	flag.BoolVar(&config.forceDimensions, "force-dimensions", defaults.forceDimensions, "Force given dimensions instead of fitting")

	flag.Float64Var(&config.lineSeparation, "separation", defaults.lineSeparation, "Separation between lines (mm)")
	flag.Float64Var(&config.nozzleGap, "gap", defaults.nozzleGap, "Nozzle gap to print target (mm)")
	flag.Float64Var(&config.plotAngle, "angle", defaults.plotAngle, "Angle to plot at (degrees)")
	flag.Float64Var(&config.plotDensity, "density", defaults.plotDensity, "Density to plot at (mm)")
	flag.Float64Var(&config.plotDirection, "direction", defaults.plotDirection, "Angle to begin plotting from (degrees)")
	flag.Float64Var(&config.speedBlack, "speed-black", defaults.speedBlack, "Speed to achieve black (mm/s)")
	flag.Float64Var(&config.speedWhite, "speed-white", defaults.speedWhite, "Minimum speed to achieve white (mm/s)")
	flag.Float64Var(&config.speedCoefficient, "speed-coefficient", defaults.speedCoefficient, "Coefficient to tune speed curve")

	flag.StringVar(&config.plotStart, "print-start", defaults.plotStart, "Print start G-Code")
	flag.StringVar(&config.plotEnd, "print-end", defaults.plotEnd, "Print end G-Code")

	flag.BoolVar(&config.drawImage, "image", defaults.drawImage, "Draw an image of the plot path")

	flag.Parse()

	inputFile, err := os.Open(imageFilename)
	errorFail(err)

	reader := bufio.NewReader(inputFile)

	imageData, _, err := image.Decode(reader)
	errorFail(err)

	bounds := imageData.Bounds()

	sourceSize = rectangle{
		float64(bounds.Max.X - bounds.Min.X),
		float64(bounds.Max.Y - bounds.Min.Y),
	}

	imageScale = scale(sourceSize, config.size, config.forceDimensions)

	path := plotPath(imageData)

	if config.drawImage {

		min := pixel{0, 0}.toPosition()
		max := path[len(path)-1]

		c := canvas.New(max[0]-min[0], max[1]-min[1])

		ctx := canvas.NewContext(c)

		ctx.SetFillColor(colornames.White)

		ctx.DrawPath(0, 0, canvas.Rectangle(c.Size()))

		p := &canvas.Path{}

		p.MoveTo(path[0][0], path[0][1])

		ctx.SetStrokeColor(color.RGBA{64, 64, 64, 128})
		ctx.SetStrokeWidth(0.1)

		for i := 0; i < len(path); i++ {

			p.LineTo(path[i][0], path[i][1])

		}

		p.Close()

		ctx.DrawPath(0, 0, p)

		// Rasterize the canvas and write to a PNG file with 3.2 dots-per-mm (320x320 px)
		err := renderers.Write("path.png", c, canvas.DPI(500))
		errorFail(err)

	}

}

// toPosition
// Converts a pixel into a point in mm
func (postion pixel) toPosition() point {
	return point{postion[0]*imageScale[0] + config.position[0], postion[1]*imageScale[1] + config.position[1]}
}

func degreesToRadAbs(deg float64) float64 {
	return (math.Pi / 180) * float64((int(deg) % 360))
}

func imageToPlotAngle(plotRad float64) float64 {
	return math.Atan(imageScale[1] / imageScale[0] * math.Tan(plotRad))
}

func dirSign(dir bool) float64 {
	if dir {
		return 1
	} else {
		return -1
	}
}

func pointSeparation(pt1, pt2 point) float64 {
	return math.Sqrt(math.Pow(pt1[0]-pt2[0], 2) + math.Pow(pt1[1]-pt2[1], 2))
}

func (pt point) distOnAngle(distance, angle float64, direction bool, boundsX [2]float64, boundsY [2]float64) point {

	var points [3]point

	sign := dirSign(direction)

	arrint := int((sign + 1) / 2)

	points = [3]point{
		point{ // Wall is boundary
			boundsX[arrint],
			pt[1] + sign*(boundsX[arrint]-pt[0])/math.Tan(angle),
		},
		point{ // Top/Bottom is boundary
			pt[0] + sign*(pt[1]-boundsY[1-arrint])*math.Tan(angle),
			boundsY[1-arrint],
		},
		point{ // Distance is boundary
			pt[0] + sign*math.Sin(angle)*distance,
			pt[1] - sign*math.Cos(angle)*distance,
		},
	}

	target := points[2]

	if target[0] > boundsX[1] || target[0] < boundsX[0] || target[1] > boundsY[1] || target[1] < boundsY[0] {
		for i := 0; i < 2; i++ {
			sep := pointSeparation(pt, points[i])
			if sep <= distance {
				distance = sep
				target = points[i]
			}
		}
	}

	return target

}

func distanceAtAngle(pt1, pt2 point, angle float64) float64 {
	var left, right, top, bottom float64

	if pt1[0] < pt2[0] {
		left = pt1[0]
		right = pt2[0]
	} else {
		left = pt2[0]
		right = pt1[0]
	}

	if pt1[1] < pt2[1] {
		bottom = pt1[1]
		top = pt2[1]
	} else {
		bottom = pt2[1]
		top = pt1[1]
	}

	diagonal := math.Sqrt(math.Pow(right-left, 2) + math.Pow(top-bottom, 2))

	switch {
	case angle <= math.Pi/2:
		angle = angle
	case angle <= 2*math.Pi/2:
		angle = math.Pi - angle
	case angle <= 3*math.Pi/2:
		angle = angle - math.Pi
	case angle <= 4*math.Pi/2:
		angle = 2*math.Pi - angle
	default:
		errorFail(fmt.Errorf("Something went wrong with the angle"))
	}

	return diagonal * math.Cos(angle)

}

func pointComplete(gap, angle float64) float64 {

	var sign float64

	switch {
	case angle <= math.Pi/2:
		angle = angle
		sign = 1
	case angle <= 2*math.Pi/2:
		angle = math.Pi - angle
		sign = -1
	case angle <= 3*math.Pi/2:
		angle = angle - math.Pi
		sign = -1
	case angle <= 4*math.Pi/2:
		angle = 2*math.Pi - angle
		sign = 1
	default:
		errorFail(fmt.Errorf("Something went wrong with the angle"))
	}

	return sign * gap / math.Tan(angle)

}

// Does not include pt1 and pt2
func interpolate(pt1, pt2 point, stepsize float64) []point {

	distance := pointSeparation(pt1, pt2)

	steps := distance / stepsize

	var nSteps int

	if steps-float64(int(steps)) == 0 {
		nSteps = int(steps)
	} else {
		nSteps = int(steps) + 1
	}

	pointSet := []point{}

	dist := [2]float64{
		(pt2[0] - pt1[0]) / float64(nSteps),
		(pt2[1] - pt1[1]) / float64(nSteps),
	}

	for i := 0; i < nSteps; i++ {
		pointSet = append(pointSet, point{dist[0] * float64(i), dist[1] * float64(i)})
	}

	return pointSet

}

// func extendLine(pt point, top, bottom, left, right, angle float64) [2]point {

// 	var sign float64

// 	if pt[0] > right || pt[0] < left || pt[1] > top || pt[1] < bottom {
// 		errorFail(fmt.Errorf("Center point out of bounds"))
// 	}

// 	leftCandidates := [2]point{
// 		point{ // Using left border
// 			left,
// 			math.Tan(angle) * (pt[0] - left),
// 		},
// 		point{ // Using top border
// 			(top - pt[1]) / math.Tan(angle),
// 			top,
// 		},
// 	}

// 	rightCandidates := [2]point{
// 		point{ // Usoing right border
// 			right,
// 			-math.Tan(angle) * (right - pt[0]),
// 		},
// 		point{ // Using bottom border

// 		},
// 	}

// 	return

// }

// func plotPathNew(data image.Image) (trace path) {

// 	var first, last point

// 	type band struct {
// 		ends [2]point
// 	}

// 	bottomLeft := pixel{0, 0}.toPosition()
// 	topRight := pixel(sourceSize).toPosition()

// 	left := bottomLeft[0]
// 	right := topRight[0]
// 	top := topRight[1]
// 	bottom := bottomLeft[1]

// 	bottomRight := point{right, bottom}
// 	topLeft := point{left, top}

// 	angleRad := degreesToRadAbs(config.plotAngle)

// 	switch {
// 	case angleRad <= math.Pi/2:
// 		first = bottomLeft
// 		last = topRight
// 	case angleRad <= 2*math.Pi/2:
// 		first = bottomRight
// 		last = topLeft
// 	case angleRad <= 3*math.Pi/2:
// 		first = topRight
// 		last = bottomLeft
// 	case angleRad <= 4*math.Pi/2:
// 		first = topLeft
// 		last = bottomRight
// 	default:
// 		errorFail(fmt.Errorf("something went wrong with the angle"))
// 	}

// 	gap := [2]float64{config.lineSeparation / math.Cos(angleRad), config.lineSeparation / math.Sin(angleRad)}

// 	distance := distanceAtAngle(first, last, angleRad)

// 	if distance <= config.lineSeparation {
// 		errorFail(fmt.Errorf("line separation must be less than diagonal distance"))
// 	}

// 	centerLine := interpolate(first, last, config.lineSeparation)

// 	nLines := len(centerLine)

// 	if nLines == 0 {
// 		centerLine = []point{point{(last[0] + first[0]) / 2, (last[1] + first[1]) / 2}}
// 		nLines = 1
// 	}

// 	bands := []band{}

// 	for i := 0; i < nLines; i++ {

// 		if len(bands) > 0 {

// 		}

// 		//

// 		bands = append(bands, band{})
// 	}

// 	return nil

// }

func plotPath(data image.Image) (trace path) {

	// TODO Use starting direction

	start := pixel{0, 0}.toPosition()
	end := pixel(sourceSize).toPosition()

	angleRad := degreesToRadAbs(config.plotAngle)

	gap := [2]float64{config.lineSeparation / math.Cos(angleRad), config.lineSeparation / math.Sin(angleRad)}

	boundsVertical := [2]float64{start[1], end[1]}
	boundsHorizontal := [2]float64{start[0], end[0]}

	var position = start

	var onWall = false

	var direction = false

	var newPos point

	for {

		trace = append(trace, newPos)

		position = newPos

		if position == end {
			fmt.Println("done")
			// Done
			break
		} else if ((position[0] == boundsHorizontal[0]) || (position[0] == boundsHorizontal[1])) && !onWall {
			onWall = true
			// On either left or right side
			if boundsVertical[1]-position[1] <= gap[1] {
				// If close to the top
				newPos[1] = boundsVertical[1]
			} else {
				// If not close
				newPos[1] = position[1] + gap[1]
			}
			direction = !direction
		} else if ((position[1] == boundsVertical[0]) || (position[1] == boundsVertical[1])) && !onWall {
			onWall = true
			// On either top or bottom
			if boundsHorizontal[1]-position[0] <= gap[0] {
				// If close to the right side
				newPos[0] = boundsHorizontal[1]
			} else {
				// If not too close
				newPos[0] = position[0] + gap[0]
			}
			direction = !direction
		} else {
			onWall = false
			newPos = position.distOnAngle(config.plotDensity, angleRad, direction, boundsHorizontal, boundsVertical)
		}

		fmt.Println(newPos)

	}

	return trace

}
