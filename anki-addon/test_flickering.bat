@echo off
setlocal enabledelayedexpansion

echo ============================================================
echo   Qt WebEngine Flickering Test Launcher
echo   (Testing backdrop-filter compositor issues)
echo ============================================================
echo.
echo Select a test scenario:
echo.
echo   [0] Baseline - No flags (default Anki behavior)
echo   [1] Disable GPU compositing only
echo   [2] Disable GPU entirely
echo   [3] Disable GPU rasterization only
echo   [4] Disable 2D canvas acceleration
echo   [5] Disable WebGL
echo   [6] Disable both 2D canvas and WebGL
echo   [7] Software rendering (SwiftShader)
echo   [8] Disable compositing + rasterization
echo   [9] Nuclear option (disable everything GPU-related)
echo.
set /p choice="Enter choice [0-9]: "

set "FLAGS="

if "%choice%"=="0" (
    echo.
    echo [Baseline] No custom flags - using Anki defaults
    set "FLAGS="
)
if "%choice%"=="1" (
    echo.
    echo [Test 1] Disabling GPU compositing...
    set "FLAGS=--disable-gpu-compositing"
)
if "%choice%"=="2" (
    echo.
    echo [Test 2] Disabling GPU entirely...
    set "FLAGS=--disable-gpu"
)
if "%choice%"=="3" (
    echo.
    echo [Test 3] Disabling GPU rasterization...
    set "FLAGS=--disable-gpu-rasterization"
)
if "%choice%"=="4" (
    echo.
    echo [Test 4] Disabling 2D canvas acceleration...
    set "FLAGS=--disable-accelerated-2d-canvas"
)
if "%choice%"=="5" (
    echo.
    echo [Test 5] Disabling WebGL...
    set "FLAGS=--disable-webgl"
)
if "%choice%"=="6" (
    echo.
    echo [Test 6] Disabling 2D canvas + WebGL...
    set "FLAGS=--disable-accelerated-2d-canvas --disable-webgl"
)
if "%choice%"=="7" (
    echo.
    echo [Test 7] Software rendering via SwiftShader...
    set "FLAGS=--disable-gpu --use-gl=swiftshader"
)
if "%choice%"=="8" (
    echo.
    echo [Test 8] Disabling compositing + rasterization...
    set "FLAGS=--disable-gpu-compositing --disable-gpu-rasterization"
)
if "%choice%"=="9" (
    echo.
    echo [Test 9] Nuclear - disabling all GPU features...
    set "FLAGS=--disable-gpu --disable-gpu-compositing --disable-gpu-rasterization --disable-accelerated-2d-canvas --disable-webgl"
)

echo.
echo Setting QTWEBENGINE_CHROMIUM_FLAGS=%FLAGS%
echo.
echo Launching Anki...
echo ============================================================

set "QTWEBENGINE_CHROMIUM_FLAGS=%FLAGS%"

REM Adjust path to your Anki installation
start "" "C:\Program Files\Anki\anki.exe"

echo.
echo Anki launched. Test the UI for flickering, then close and try another scenario.
pause
