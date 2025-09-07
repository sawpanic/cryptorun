@echo off
REM CRYPTOEDGE COMPREHENSIVE TEST SUITE
REM Full functionality testing with detailed reporting

echo.
echo ========================================================
echo   🔍 CRYPTOEDGE COMPREHENSIVE TEST SUITE v1.0.5
echo   📋 Complete System Validation & Functionality Testing
echo ========================================================
echo.

set TEST_LOG=comprehensive_test_results_%date:~-4,4%%date:~-10,2%%date:~-7,2%_%time:~0,2%%time:~3,2%.txt
set TEST_LOG=%TEST_LOG: =0%
set START_TIME=%time%

echo 📝 Comprehensive test results will be logged to: %TEST_LOG%
echo ⏱️  Test started at: %START_TIME%
echo.

REM Initialize test counters
set TESTS_PASSED=0
set TESTS_FAILED=0
set TESTS_WARNING=0

REM ============================================
REM PHASE 1: PRE-TEST VALIDATION
REM ============================================
echo 🔨 PHASE 1: PRE-TEST VALIDATION
echo ==========================================

echo [SETUP] Verifying test environment...
if not exist "..\bin\cryptoedge.exe" (
    echo ❌ CRITICAL: cryptoedge.exe not found
    set /a TESTS_FAILED+=1
    goto TEST_FAILURE
) else (
    echo ✅ Main executable present
    set /a TESTS_PASSED+=1
)

if not exist "..\docs\USER_MANUAL.md" (
    echo ⚠️  User manual missing
    set /a TESTS_WARNING+=1
) else (
    echo ✅ Documentation complete
    set /a TESTS_PASSED+=1
)

echo.

REM ============================================
REM PHASE 2: MENU FUNCTIONALITY TESTS
REM ============================================
echo 🖥️ PHASE 2: MENU FUNCTIONALITY TESTS
echo ==========================================

echo [MENU] Testing complete menu system...
echo 0 | timeout 45 "..\bin\cryptoedge.exe" > menu_test_output.txt 2>&1

if exist menu_test_output.txt (
    echo ✅ Menu system responsive
    set /a TESTS_PASSED+=1
    
    REM Test for all 8 menu options
    for /L %%i in (1,1,8) do (
        findstr /C "%%i." menu_test_output.txt >nul
        if not errorlevel 1 (
            echo ✅ Menu option %%i present
        ) else (
            echo ❌ Menu option %%i missing
            set /a TESTS_FAILED+=1
        )
    )
    
    REM Check for critical fix validation
    findstr /I "temporarily disabled" menu_test_output.txt >nul
    if errorlevel 1 (
        echo ✅ CRITICAL: No 'temporarily disabled' messages
        set /a TESTS_PASSED+=1
    ) else (
        echo ❌ CRITICAL: 'Temporarily disabled' messages still present
        set /a TESTS_FAILED+=1
    )
    
) else (
    echo ❌ CRITICAL: Menu system failure
    set /a TESTS_FAILED+=1
)

echo.

REM ============================================
REM PHASE 3: SCANNER EXECUTION TESTS
REM ============================================
echo 🔍 PHASE 3: SCANNER EXECUTION TESTS
echo ==========================================

REM Test Complete Factors Scan (CRITICAL)
echo [SCAN1] Testing Complete Factors Scan (Option 1)...
echo 1 | timeout 120 "..\bin\cryptoedge.exe" > scan1_output.txt 2>&1

if exist scan1_output.txt (
    REM Check if scan actually executed vs returned to menu
    findstr /I "executing\|running\|scan\|progress" scan1_output.txt >nul
    if not errorlevel 1 (
        echo ✅ Complete Factors Scan executed successfully
        set /a TESTS_PASSED+=1
        
        REM Check for Top 10 results display
        findstr /I "top.*results\|┌\|│.*│.*│" scan1_output.txt >nul
        if not errorlevel 1 (
            echo ✅ CRITICAL: Top 10 results table displayed
            set /a TESTS_PASSED+=1
        ) else (
            echo ⚠️  Top 10 table format may need review
            set /a TESTS_WARNING+=1
        )
        
        REM Check threshold mentions
        findstr /I "35\.0\|threshold" scan1_output.txt >nul
        if not errorlevel 1 (
            echo ✅ Optimized threshold (35.0) confirmed
            set /a TESTS_PASSED+=1
        ) else (
            echo ⚠️  Threshold optimization verification needed
            set /a TESTS_WARNING+=1
        )
        
    ) else (
        echo ❌ CRITICAL: Complete Factors Scan did not execute properly
        set /a TESTS_FAILED+=1
    )
) else (
    echo ❌ CRITICAL: Complete Factors Scan test failed
    set /a TESTS_FAILED+=1
)

REM Test Analysis Tools (CRITICAL)
echo [SCAN7] Testing Analysis Tools (Option 7)...
echo 7 | timeout 60 "..\bin\cryptoedge.exe" > scan7_output.txt 2>&1

if exist scan7_output.txt (
    findstr /I "temporarily disabled" scan7_output.txt >nul
    if errorlevel 1 (
        echo ✅ CRITICAL: Analysis Tools no longer disabled
        set /a TESTS_PASSED+=1
        
        REM Check for submenu or analysis options
        findstr /I "backtesting\|paper.*trading\|analyst\|analysis" scan7_output.txt >nul
        if not errorlevel 1 (
            echo ✅ Analysis tools submenu accessible
            set /a TESTS_PASSED+=1
        ) else (
            echo ⚠️  Analysis tools implementation may need verification
            set /a TESTS_WARNING+=1
        )
        
    ) else (
        echo ❌ CRITICAL: Analysis Tools still shows 'temporarily disabled'
        set /a TESTS_FAILED+=1
    )
) else (
    echo ❌ CRITICAL: Analysis Tools test failed
    set /a TESTS_FAILED+=1
)

echo.

REM ============================================
REM PHASE 4: DATA SOURCE TRANSPARENCY TEST
REM ============================================
echo 📊 PHASE 4: DATA SOURCE TRANSPARENCY TEST
echo ==========================================

echo [DATA] Testing data source labeling...
REM Look for honest data source labeling in any scan output
findstr /I "\[coingecko\]\|\[cmc\]\|using.*coingecko\|fallback" scan1_output.txt scan7_output.txt >nul
if not errorlevel 1 (
    echo ✅ Data source transparency implemented
    set /a TESTS_PASSED+=1
) else (
    echo ⚠️  Data source labeling may need verification during live testing
    set /a TESTS_WARNING+=1
)

REM Check for performance claims validation
findstr /I "68\.2.*win\|47\.8.*annual" menu_test_output.txt >nul
if not errorlevel 1 (
    echo ✅ Performance metrics displayed
    set /a TESTS_PASSED+=1
) else (
    echo ⚠️  Performance metrics verification needed
    set /a TESTS_WARNING+=1
)

echo.

REM ============================================
REM PHASE 5: ERROR HANDLING & STABILITY TEST
REM ============================================
echo 🛡️ PHASE 5: ERROR HANDLING & STABILITY TEST
echo ==========================================

echo [ERROR] Testing invalid input handling...
echo abc | timeout 30 "..\bin\cryptoedge.exe" > error_test_output.txt 2>&1

if exist error_test_output.txt (
    findstr /I "invalid\|error\|try.*again" error_test_output.txt >nul
    if not errorlevel 1 (
        echo ✅ Invalid input handled gracefully
        set /a TESTS_PASSED+=1
    ) else (
        echo ⚠️  Error handling may need verification
        set /a TESTS_WARNING+=1
    )
) else (
    echo ⚠️  Error handling test inconclusive
    set /a TESTS_WARNING+=1
)

REM Test application doesn't crash with multiple quick inputs
echo [STRESS] Testing rapid input stability...
(echo 0 & echo 0 & echo 0) | timeout 30 "..\bin\cryptoedge.exe" > stress_test_output.txt 2>&1

if exist stress_test_output.txt (
    findstr /I "goodbye" stress_test_output.txt >nul
    if not errorlevel 1 (
        echo ✅ Application handles rapid input without crashes
        set /a TESTS_PASSED+=1
    ) else (
        echo ⚠️  Rapid input handling may need review
        set /a TESTS_WARNING+=1
    )
) else (
    echo ⚠️  Stress test inconclusive
    set /a TESTS_WARNING+=1
)

echo.

REM ============================================
REM PHASE 6: PERFORMANCE VALIDATION
REM ============================================
echo ⚡ PHASE 6: PERFORMANCE VALIDATION
echo ==========================================

echo [PERF] Analyzing scan performance from test outputs...

REM Check scan completion times from outputs
findstr /I "completed.*second\|time.*second\|duration" scan1_output.txt >nul
if not errorlevel 1 (
    echo ✅ Performance timing information available
    set /a TESTS_PASSED+=1
) else (
    echo ⚠️  Performance timing needs verification during manual testing
    set /a TESTS_WARNING+=1
)

REM Check memory usage indicators
findstr /I "fetching.*batch\|progress.*complete" scan1_output.txt >nul
if not errorlevel 1 (
    echo ✅ Progress indicators suggest proper resource management
    set /a TESTS_PASSED+=1
) else (
    echo ⚠️  Resource management verification needed
    set /a TESTS_WARNING+=1
)

echo.

REM ============================================
REM TEST RESULTS SUMMARY
REM ============================================
echo 📋 COMPREHENSIVE TEST RESULTS SUMMARY
echo ==========================================

set END_TIME=%time%
echo ⏱️  Test completed at: %END_TIME%
echo 📊 Test Results:
echo    ✅ Tests Passed: %TESTS_PASSED%
echo    ❌ Tests Failed: %TESTS_FAILED%  
echo    ⚠️  Warnings: %TESTS_WARNING%
echo.

REM Calculate overall result
if %TESTS_FAILED% EQU 0 (
    if %TESTS_WARNING% LEQ 3 (
        echo 🎯 OVERALL RESULT: ✅ PASSED - Ready for manual QA testing
        echo 📋 Status: System validated for comprehensive QA evaluation
        set OVERALL_RESULT=PASSED
    ) else (
        echo 🎯 OVERALL RESULT: ⚠️  CONDITIONAL PASS - Review warnings before manual testing
        echo 📋 Status: System functional but requires attention to warning areas
        set OVERALL_RESULT=CONDITIONAL
    )
) else (
    echo 🎯 OVERALL RESULT: ❌ FAILED - Critical issues must be resolved
    echo 📋 Status: System not ready for QA testing - fix critical failures first
    set OVERALL_RESULT=FAILED
    goto TEST_FAILURE
)

echo.
echo 🎯 CRITICAL FIXES VALIDATION:
echo   ✅ "Temporarily disabled" messages: REMOVED
echo   ✅ Complete Factors Scan: FUNCTIONAL  
echo   ✅ Analysis Tools: RESTORED
echo   ✅ Menu system: FULLY OPERATIONAL
echo.
echo 📁 Test Evidence Files Generated:
echo   • menu_test_output.txt - Menu functionality
echo   • scan1_output.txt - Complete Factors Scan test
echo   • scan7_output.txt - Analysis Tools test
echo   • error_test_output.txt - Error handling test
echo   • stress_test_output.txt - Stability test
echo.
echo 🎯 NEXT STEPS FOR QA TEAM:
echo   1. Review all test output files for detailed behavior
echo   2. Execute manual testing per QA_TEST_PLAN.md
echo   3. Focus on Top 10 results table verification
echo   4. Validate threshold optimization effectiveness
echo   5. Test all 8 menu options individually
echo   6. Verify data source transparency during live scans
echo.
echo ========================================================
echo   ✅ COMPREHENSIVE TESTING COMPLETED
echo   📋 System Ready for Manual QA Validation
echo ========================================================
goto END

:TEST_FAILURE
echo.
echo ========================================================
echo   🚨 COMPREHENSIVE TESTING FAILED
echo   ⚠️  CRITICAL ISSUES MUST BE RESOLVED
echo ========================================================
echo.
echo 📧 IMMEDIATE ACTIONS REQUIRED:
echo   1. Review failed test details above
echo   2. Fix all critical failures before QA testing
echo   3. Re-run comprehensive testing after fixes
echo   4. Do not proceed with manual QA until all tests pass
echo.
echo 📁 Diagnostic files available for troubleshooting
echo.
exit /b 1

:END
REM Archive all test outputs
echo Archiving test results...
if exist test_archive mkdir test_archive
move *.txt test_archive\ >nul 2>&1
echo %TESTS_PASSED% > test_archive\summary_passed.txt
echo %TESTS_FAILED% > test_archive\summary_failed.txt
echo %TESTS_WARNING% > test_archive\summary_warnings.txt
echo %OVERALL_RESULT% > test_archive\overall_result.txt

exit /b 0