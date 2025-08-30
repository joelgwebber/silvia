#!/bin/bash
# Create simple placeholder icons for the extension
# These are just colored squares with "S" for Silvia

# Create 16x16 icon
echo '<?xml version="1.0" encoding="UTF-8"?>
<svg width="16" height="16" xmlns="http://www.w3.org/2000/svg">
  <rect width="16" height="16" fill="#667eea" rx="3"/>
  <text x="8" y="12" font-family="Arial" font-size="12" fill="white" text-anchor="middle">S</text>
</svg>' > icon-16.svg

# Create 48x48 icon
echo '<?xml version="1.0" encoding="UTF-8"?>
<svg width="48" height="48" xmlns="http://www.w3.org/2000/svg">
  <rect width="48" height="48" fill="#667eea" rx="8"/>
  <text x="24" y="34" font-family="Arial" font-size="32" fill="white" text-anchor="middle">S</text>
</svg>' > icon-48.svg

# Create 128x128 icon
echo '<?xml version="1.0" encoding="UTF-8"?>
<svg width="128" height="128" xmlns="http://www.w3.org/2000/svg">
  <rect width="128" height="128" fill="#667eea" rx="16"/>
  <text x="64" y="90" font-family="Arial" font-size="80" fill="white" text-anchor="middle">S</text>
</svg>' > icon-128.svg

echo "SVG icons created. Note: Chrome requires PNG format."
echo "To convert to PNG, you can use ImageMagick or an online converter."
echo "For now, these SVG files serve as placeholders."

# Create simple PNG placeholders using base64 encoded 1x1 purple pixel
# These are minimal valid PNGs that will show as purple squares
echo -n "iVBORw0KGgoAAAANSUhEUgAAABAAAAAQCAYAAAAf8/9hAAAABHNCSVQICAgIfAhkiAAAAAlwSFlzAAAAbwAAAG8B8aLcQwAAABl0RVh0U29mdHdhcmUAd3d3Lmlua3NjYXBlLm9yZ5vuPBoAAABFSURBVDiNY2Bg+M/AwMDAYGRkxMDAwMDw//9/BpzgPwMDA4ORkRGGj1UzCwMDAwMDy3+cmlmwacamGZtmbJqxacamGQIAAPVfB0n3VNsVAAAAAElFTkSuQmCC" | base64 -d > icon-16.png
echo -n "iVBORw0KGgoAAAANSUhEUgAAADAAAAAwCAYAAABXAvmHAAAABHNCSVQICAgIfAhkiAAAAAlwSFlzAAAAbwAAAG8B8aLcQwAAABl0RVh0U29mdHdhcmUAd3d3Lmlua3NjYXBlLm9yZ5vuPBoAAABFSURBVGiB7dAxAQAACAOgaReV/93gJwcPMAACBAQECAgQECAgQECAgAABAQICBAQICBAQICBAQICAgAABAQICBAQIfAcLYwAw7cO/VQAAAABJRU5ErkJggg==" | base64 -d > icon-48.png
echo -n "iVBORw0KGgoAAAANSUhEUgAAAIAAAACACAYAAADDPmHLAAAABHNCSVQICAgIfAhkiAAAAAlwSFlzAAAAbwAAAG8B8aLcQwAAABl0RVh0U29mdHdhcmUAd3d3Lmlua3NjYXBlLm9yZ5vuPBoAAABFSURBVHic7cExAQAAAMKg9U/tbwagAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAOBmAMAAAfTfKbEAAAAASUVORK5CYII=" | base64 -d > icon-128.png

echo "Basic PNG placeholders created."
