# File Explorer API Design Document

## Overview
This RESTful API manages files and folders within a specified root directory (e.g., `/home/user/docs`). All paths are relative to the root, URL-encoded, and operations include listing, reading, modifying, deleting, uploading, downloading, and creating folders. The base URL is `/api/fs`.

## Assumptions
- Paths resolve relative to the root directory.
- Authentication/authorization is out of scope (implement with API keys/JWT).
- Error responses: JSON `{"error": "message"}` with HTTP status codes (400, 404, 500, etc.).
- File content: Binary or UTF-8 text; MIME types inferred.
- Folders: Recursive delete is optional; no direct upload (use create folder).
- No versioning or locking.

## API Endpoints

### 1. GET /api/fs/<path>
- **Description**: Performs different actions based on the path type and query parameters:
  - **Directory**: Lists files and subfolders.
  - **File**: Returns raw file content.
  - **File with `download=true`**: Triggers file download with appropriate headers.
- **URL**: `/api/fs/<path>`
  - `<path>`: Relative path to a file or directory (e.g., `folder/file.txt` or `folder`). Empty path (`/api/fs/`) targets the root.
- **Query Params**:
  - `download`: boolean (default: false) – Triggers download mode for files.
- **Request Body**: None.
- **Response**:
  - **Directory Listing** (200 OK, `application/json`):
    ```json
    [
      {"name": "file.txt", "type": "file", "size": 1024, "lastModified": "2025-10-24T12:00:00Z"},
      {"name": "subfolder", "type": "directory", "size": 0, "lastModified": "2025-10-24T11:00:00Z"}
    ]
    ```
  - **File Content** (200 OK, MIME type inferred, e.g., `text/plain`):
    - Body: Raw file content (binary or text).
  - **File Download** (200 OK, MIME type inferred):
    - Headers: `Content-Disposition: attachment; filename="<name>"`.
    - Body: Raw file content.
- **Errors**:
  - 400: Path is a directory (for download) or not a directory (for listing).
  - 404: Path not found.

### 2. POST /api/fs/<path>
- **Description**: Performs different actions based on the path and body:
  - **Directory**: Uploads files to `<path>`.
  - **Parent Directory**: Creates a new folder under `<path>`.
- **URL**: `/api/fs/<path>`
  - `<path>`: Relative path to a directory (e.g., `folder`). Empty path targets the root.
- **Query Params**:
  - `overwrite`: boolean (default: false) – For uploads, overwrites existing files.
- **Request Body**:
  - **Upload**: `multipart/form-data`, field `files` (array of files).
  - **Create Folder**: JSON `{"name": "<new_folder_name>"}`.
- **Response**:
  - **Upload** (201 Created, `application/json`):
    ```json
    {"success": true, "uploaded": ["file1.txt", "file2.jpg"]}
    ```
  - **Create Folder** (201 Created, `application/json`):
    ```json
    {"success": true, "path": "<full_new_path>"}
    ```
- **Errors**:
  - 400: Invalid body (e.g., missing `name` for folder creation).
  - 404: Directory/parent not found.
  - 409: Conflict (file exists without overwrite, or folder exists).

### 3. PUT /api/fs/<path>
- **Description**: Modifies a file or folder:
  - **File**: Updates content at `<path>`.
  - **Directory**: Renames folder at `<path>`.
- **URL**: `/api/fs/<path>`
  - `<path>`: Relative path to a file or directory (e.g., `folder/file.txt` or `folder`).
- **Query Params**:
  - `new_name`: string (required for directory rename).
- **Request Body**:
  - **File**: Raw new content (binary or text, with appropriate `Content-Type`).
  - **Directory**: None (use `new_name` query param).
- **Response**: 200 OK, `application/json`:
  ```json
  {"success": true}
  ```
- **Errors**:
  - 400: Invalid operation (e.g., body for directory, missing `new_name` for directory).
  - 404: Path not found.

### 4. DELETE /api/fs/<path>
- **Description**: Deletes the file or folder at `<path>`.
- **URL**: `/api/fs/<path>`
  - `<path>`: Relative path to a file or directory (e.g., `folder/file.txt` or `folder`).
- **Query Params**:
  - `recursive`: boolean (default: false) – For directories, deletes contents recursively.
- **Request Body**: None.
- **Response**: 200 OK, `application/json`:
  ```json
  {"success": true}
  ```
- **Errors**:
  - 400: Directory not empty (without `recursive=true`).
  - 404: Path not found.

## Implementation Notes
- **Path Resolution**: Combine root with relative path; prevent traversal (e.g., block `../`).
- **File System**: Use OS-level operations (e.g., Node.js `fs` module).
- **Security**: Add authentication; validate paths and permissions.
- **MIME Types**: Infer from file extensions or content.
- **Scalability**: Stream large files for read/download/upload.

## Future Enhancements
- Search endpoint (e.g., `/api/fs/search?q=<query>`).
- Dedicated move/rename endpoint.
- Metadata support (e.g., permissions, tags).