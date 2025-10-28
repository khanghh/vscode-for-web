/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/

import {
  Disposable,
  Event,
  EventEmitter,
  FileChangeEvent,
  FileChangeType,
  FileStat,
  FileSystemError,
  FileSystemProvider,
  FileType,
  Position,
  Progress,
  Range,
  Uri,
  CancellationToken,
  ProviderResult,
  workspace,
} from 'vscode';

export class RemoteFS implements FileSystemProvider {
  static scheme = 'remotefs';

  private readonly disposable: Disposable;
  private baseUrl: string;

  constructor() {
    this.baseUrl = `http://localhost:3000/api/v1/fs`;
    this.disposable = Disposable.from(
      workspace.registerFileSystemProvider(RemoteFS.scheme, this, { isCaseSensitive: true }),
    );
  }

  dispose() {
    this.disposable?.dispose();
  }

  // --- manage file metadata

  private async httpRequest(method: string, url: string, body?: string | ArrayBuffer, responseType: 'text' | 'arraybuffer' = 'text'): Promise<Response> {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open(method, url, true);
      xhr.responseType = responseType;
      xhr.onload = () => {
        resolve({
          ok: xhr.status >= 200 && xhr.status < 300,
          status: xhr.status,
          statusText: xhr.statusText,
          json: () => Promise.resolve(JSON.parse(xhr.responseText || '{}')),
          arrayBuffer: () => Promise.resolve(xhr.response),
          text: () => Promise.resolve(xhr.responseText || ''),
        } as any);
      };
      xhr.onerror = () => reject(new Error('Network error'));
      if (body) {
        if (typeof body === 'string') {
          xhr.setRequestHeader('Content-Type', 'application/json');
          xhr.send(body);
        } else {
          xhr.setRequestHeader('Content-Type', 'application/octet-stream');
          xhr.send(body);
        }
      } else {
        xhr.send();
      }
    });
  }

  async stat(uri: Uri): Promise<FileStat> {
    const path = uri.path.substring(1); // remove leading /
    const url = `${this.baseUrl}/${path}?stat=true`;
    const response = await this.httpRequest('GET', url, undefined, 'text');
    if (!response.ok) {
      throw this.mapError(response);
    }
    const data = await response.json();
    return {
      type: data.type === 'directory' ? FileType.Directory : FileType.File,
      ctime: Date.parse(data.lastModified),
      mtime: Date.parse(data.lastModified),
      size: data.size,
    };
  }

  async readDirectory(uri: Uri): Promise<[string, FileType][]> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await this.httpRequest('GET', url, undefined, 'text');
    if (!response.ok) {
      throw this.mapError(response);
    }
    const data: any[] = await response.json();
    return data.map(item => [
      item.name,
      item.type === 'directory' ? FileType.Directory : FileType.File
    ] as [string, FileType]);
  }

  // --- manage file contents

  async readFile(uri: Uri): Promise<Uint8Array> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await this.httpRequest('GET', url, undefined, 'text');
    if (!response.ok) {
      throw this.mapError(response);
    }
    const base64 = await response.text();
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) {
      bytes[i] = binary.charCodeAt(i);
    }
    return bytes;
  }

  async writeFile(uri: Uri, content: Uint8Array, options: { create: boolean, overwrite: boolean }): Promise<void> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const base64 = btoa(String.fromCharCode(...Array.from(content)));
    const response = await this.httpRequest('PUT', url, base64);
    if (!response.ok) {
      throw this.mapError(response);
    }
  }

  // --- manage files/folders

  async rename(oldUri: Uri, newUri: Uri, options: { overwrite: boolean }): Promise<void> {
    const oldPath = oldUri.path.substring(1);
    const newName = newUri.path.split('/').pop()!;
    const url = `${this.baseUrl}/${oldPath}?new_name=${encodeURIComponent(newName)}`;
    const response = await this.httpRequest('PUT', url);
    if (!response.ok) {
      throw this.mapError(response);
    }
  }

  async delete(uri: Uri): Promise<void> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await this.httpRequest('DELETE', url);
    if (!response.ok) {
      throw this.mapError(response);
    }
  }

  async createDirectory(uri: Uri): Promise<void> {
    const pathParts = uri.path.split('/');
    const parentPath = pathParts.slice(1, -1).join('/');
    const name = pathParts[pathParts.length - 1];
    const url = `${this.baseUrl}/${parentPath}`;
    const response = await this.httpRequest('POST', url, JSON.stringify({ name }));
    if (!response.ok) {
      throw this.mapError(response);
    }
  }

  // --- manage file events

  private _emitter = new EventEmitter<FileChangeEvent[]>();
  private _bufferedEvents: FileChangeEvent[] = [];
  private _fireSoonHandle?: any;

  readonly onDidChangeFile: Event<FileChangeEvent[]> = this._emitter.event;

  watch(_resource: Uri): Disposable {
    // ignore, fires for all changes...
    return new Disposable(() => { });
  }

  private _fireSoon(...events: FileChangeEvent[]): void {
    this._bufferedEvents.push(...events);

    if (this._fireSoonHandle) {
      clearTimeout(this._fireSoonHandle);
    }

    this._fireSoonHandle = setTimeout(() => {
      this._emitter.fire(this._bufferedEvents);
      this._bufferedEvents.length = 0;
    }, 5);
  }

  private mapError(response: Response): Error {
    switch (response.status) {
      case 404:
        return FileSystemError.FileNotFound();
      case 400:
        return FileSystemError.FileIsADirectory();
      case 409:
        return FileSystemError.FileExists();
      default:
        return new Error(`HTTP ${response.status}: ${response.statusText}`);
    }
  }
}