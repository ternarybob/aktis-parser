# Confluence Tests Documentation

This document describes the API and UI tests for Confluence functionality in the Aktis Parser application.

## Test Files

### API Tests
Located in `tests/api/confluence_api_test.go`

### UI Tests
Located in `tests/ui/`:
- `confluence_clear_data_test.go`
- `confluence_sync_spaces_test.go`
- `confluence_get_pages_test.go`
- `confluence_page_detail_test.go`

## Test Coverage

### 1. Clear All Data (`TestConfluence_ClearAllData`)

**API Test:**
- ✅ POST `/api/data/clear-all` returns 200 OK
- ✅ Response contains success status
- ✅ Verifies spaces are cleared from database
- ✅ Verifies pages are cleared from database

**UI Test:**
- ✅ Page loads successfully
- ✅ CLEAR ALL DATA button exists and is clickable
- ✅ Confirmation dialog is handled
- ✅ Spaces list shows "No spaces in cache" message
- ✅ Pages table shows "No pages loaded" message
- ✅ Data is actually cleared from UI

### 2. Sync Spaces (`TestConfluence_SyncSpaces`)

**API Test:**
- ✅ Clears all data first
- ✅ POST `/api/spaces/refresh-cache` returns 200 OK
- ✅ Response contains "started" status
- ✅ Polls for spaces to appear (max 30 seconds)
- ✅ Verifies spaces have required fields (key, name)
- ✅ Verifies at least one space has page count > 0
- ✅ Logs individual space statistics

**UI Test:**
- ✅ Page loads successfully
- ✅ Clears data first
- ✅ GET SPACES button exists and is clickable
- ✅ Syncing starts successfully
- ✅ Polls for spaces to appear with page counts (max 30 seconds)
- ✅ Verifies at least one space has pages
- ✅ Logs detailed space statistics:
  - Total spaces
  - Spaces with pages
  - Spaces with 0 pages
  - Individual space page counts
- ✅ Fails if all spaces have 0 pages (indicates API issue)

### 3. Get Pages (`TestConfluence_GetPages`)

**API Test:**
- ✅ Gets available spaces
- ✅ Selects first space
- ✅ POST `/api/spaces/get-pages` with space keys
- ✅ Response contains "started" status
- ✅ Polls for pages to appear (max 30 seconds)
- ✅ Verifies pages have required fields (id, title)
- ✅ Verifies all pages belong to selected space

**UI Test:**
- ✅ Page loads successfully
- ✅ Waits for spaces to load
- ✅ Selects first space by clicking
- ✅ Verifies URL is updated with space key parameter
- ✅ GET PAGES button is enabled and clickable
- ✅ Pages load within timeout (max 30 seconds)
- ✅ Verifies all pages match selected space
- ✅ **URL Persistence Test:**
  - Refreshes page
  - Verifies space selection persists
  - Verifies pages reload automatically
- ✅ Screenshots captured at each step

### 4. Page Detail (`TestConfluence_PageDetail`)

**API Test:**
- ✅ Not applicable (pure UI functionality)
- ✅ Covered by page filtering API test

**UI Test:**
- ✅ Page loads successfully
- ✅ Waits for pages to be available
- ✅ Clicks first page row
- ✅ Verifies page detail box displays content
- ✅ Verifies page ID shown in detail header
- ✅ Validates JSON content is valid
- ✅ Verifies JSON contains required fields (id, title, type)
- ✅ Verifies selected row is highlighted
- ✅ Verifies URL contains page ID parameter
- ✅ **URL Persistence Test:**
  - Refreshes page
  - Verifies page detail persists
  - Verifies correct page ID is displayed
  - Verifies row highlighting persists
- ✅ Screenshots captured at each step

## Additional API Tests

### Page Filtering (`TestConfluence_PageFiltering`)
- ✅ Gets multiple spaces
- ✅ Queries pages with multiple space filters
- ✅ Verifies all returned pages belong to filtered spaces
- ✅ Tests query parameter handling

## Running Tests

### Run All Tests
```powershell
.\tests\run-tests.ps1 -Type all
```

### Run API Tests Only
```powershell
.\tests\run-tests.ps1 -Type api
```

### Run UI Tests Only
```powershell
.\tests\run-tests.ps1 -Type ui
```

### Run Specific Test
```powershell
.\tests\run-tests.ps1 -Test "TestConfluence_SyncSpaces"
```

## Test Dependencies

Tests should be run in this order for best results:

1. `TestConfluence_ClearAllData` - Clears database
2. `TestConfluence_SyncSpaces` - Populates spaces
3. `TestConfluence_GetPages` - Fetches pages for spaces
4. `TestConfluence_PageDetail` - Tests page selection and detail view

## Test Results

Test results are saved to `tests/results/run_[timestamp]/`:
- `test-results.log` - Overall test summary
- `api/test.log` - API test output
- `ui/test.log` - UI test output
- `ui/*.png` - Screenshots from UI tests

## URL Format Examples

### Space Selection
- Single space: `http://localhost:8085/confluence?spaces=ENG`
- Multiple spaces: `http://localhost:8085/confluence?spaces=ENG,DOCS`
- All spaces: `http://localhost:8085/confluence?spaces=ALL`

### Page Selection
- With space and page: `http://localhost:8085/confluence?spaces=ENG&page=98765`

## Success Criteria

### API Tests
- All endpoints return expected HTTP status codes
- Response bodies contain expected data structures
- Data persists correctly in database
- Filtering works correctly

### UI Tests
- All UI elements render correctly
- User interactions work as expected
- Data loads within acceptable timeouts
- URL parameters persist across page refreshes
- Visual feedback (highlighting, notifications) works correctly
- Screenshots show expected UI state at each step

## Failure Scenarios

Tests are designed to fail explicitly for these conditions:

1. **Sync Timeout** - Spaces don't appear within 30 seconds
2. **Zero Page Counts** - All spaces have 0 pages (indicates API issue)
3. **Wrong Space Pages** - Pages don't belong to selected space
4. **Persistence Failure** - Selection doesn't persist after refresh
5. **Invalid JSON** - Page detail JSON is malformed
6. **Missing UI Elements** - Required buttons or sections not found

## Video Recording

UI tests automatically record video (if ffmpeg is available):
- Videos saved to `tests/results/run_[timestamp]/ui/`
- Format: `test_[test_name]_[timestamp].mp4`
- Useful for debugging test failures

## Notes

- UI tests use headless Chrome by default
- Tests include automatic retry logic for network operations
- All tests clean up after themselves
- Screenshots are taken at key points for debugging
- Tests log detailed progress information
