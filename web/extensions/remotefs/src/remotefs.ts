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
  FilePermission,
} from 'vscode';
import axios from 'axios';

export class RemoteFS implements FileSystemProvider {
  static scheme = 'remotefs';

  private readonly disposable: Disposable;
  private baseUrl: string;

  constructor() {
    const globalAny: any = (typeof globalThis !== 'undefined') ? globalThis : {};
    const origin =  globalAny.origin || 'http://localhost:3000';
    this.baseUrl = `${origin.replace(/\/$/, '')}/api/v1/fs`;
    this.disposable = Disposable.from(
      workspace.registerFileSystemProvider(RemoteFS.scheme, this, { isCaseSensitive: true }),
    );
  }

  dispose() {
    this.disposable?.dispose();
  }

  // --- manage file metadata

  async stat(uri: Uri): Promise<FileStat> {
    const path = uri.path.substring(1); // remove leading /
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.get(url, {
      params: { stat: true },
      validateStatus: () => true
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    const data = response.data;
    return {
      type: data.type as FileType,
      ctime: Date.parse(data.lastModified),
      mtime: Date.parse(data.lastModified),
      size: data.size,
    };
  }

  async readDirectory(uri: Uri): Promise<[string, FileType][]> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.get(url, { validateStatus: () => true });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    const data: any[] = response.data;
    return data.map(item => [item.name, item.type] as [string, FileType]);
  }

  // --- manage file contents

  async readFile(uri: Uri): Promise<Uint8Array> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.get(url, { responseType: 'arraybuffer', validateStatus: () => true });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    return new Uint8Array(response.data as ArrayBuffer);
  }

  async writeFile(uri: Uri, content: Uint8Array, options: { create: boolean, overwrite: boolean }): Promise<void> {
    const path = uri.path.substring(1);
    const parts = path.split('/');
    const fileName = parts.pop() || '';

    const buffer = new ArrayBuffer(content.byteLength);
    new Uint8Array(buffer).set(content);

    if (options.create) {
      // if create, use POST to create empty file or upload file to parent directory path
      const parent = parts.join('/');
      const url = `${this.baseUrl}/${parent}`;
      let response;
      if (content.length > 0) {
        const form = new FormData();
        form.append('file', new Blob([buffer]), fileName);
        if (options.overwrite) {
          form.set('overwrite', 'true');
        }
        response = await axios.post(url, form, { validateStatus: () => true, });
      } else {
        const body: any = { path: fileName, type: 'file' };
        if (options.overwrite) {
          body.overwrite = true;
        }
        response = await axios.post(url, body, {
          headers: { 'Content-Type': 'application/json' },
          validateStatus: () => true,
        });
      }
      if (response.status !== axios.HttpStatusCode.Created) {
        throw this.mapError(response);
      }
      return this._fireSoon({ type: FileChangeType.Created, uri });
    }

    // default, use PUT to write content to file path
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.put(url, buffer, {
      headers: { 'Content-Type': 'application/octet-stream' },
      validateStatus: () => true
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    return this._fireSoon({ type: FileChangeType.Changed, uri });
  }

  // --- manage files/folders

  async rename(oldUri: Uri, newUri: Uri, options: { overwrite: boolean }): Promise<void> {
    const oldPath = oldUri.path.substring(1);
    const newName = newUri.path.substring(1);
    const url = `${this.baseUrl}/${oldPath}`;
    const response = await axios.patch(url, { newPath: newName, overwrite: options.overwrite }, {
      headers: { 'Content-Type': 'application/json' },
      validateStatus: () => true,
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    this._fireSoon(
      { type: FileChangeType.Deleted, uri: oldUri },
      { type: FileChangeType.Changed, uri: newUri }
    );
  }

  async delete(uri: Uri, opts: { recursive: boolean, useTrash: boolean }): Promise<void> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.delete(url, {
      params: { recursive: opts.recursive },
      validateStatus: () => true
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    this._fireSoon({ uri, type: FileChangeType.Deleted });
  }

  async createDirectory(uri: Uri): Promise<void> {
    const basename = this._basename(uri.path);
    const parent = this._dirname(uri.path);
    const url = `${this.baseUrl}/${parent}`;
    const body = { path: basename, type: 'directory' };
    const response = await axios.post(url, body, {
      headers: { 'Content-Type': 'application/json' },
      validateStatus: () => true
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    this._fireSoon({ type: FileChangeType.Created, uri });
  }

  // --- manage file events

  private _emitter = new EventEmitter<FileChangeEvent[]>();
  private _bufferedEvents: FileChangeEvent[] = [];
  private _fireSoonHandle?: any;

  readonly onDidChangeFile: Event<FileChangeEvent[]> = this._emitter.event;

  watch(_resource: Uri): Disposable {
    // ignore, fires for all changes...
    return new Disposable(() => {
      this._bufferedEvents = [];
    });
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

  private mapError(response: any): Error {
    const data = response?.data ?? {};
    const code = data?.code ?? '';

    switch (code) {
      case 'FILE_EXISTS':
        return FileSystemError.FileExists();
      case 'FILE_NOT_FOUND':
        return FileSystemError.FileNotFound();
      case 'FILE_IS_DIRECTORY':
        return FileSystemError.FileIsADirectory();
      case 'NO_PERMISSIONS':
        return FileSystemError.NoPermissions();
    }

    if (response?.status === 500) {
      return new Error('Internal server error');
    }

    const message = data?.error ?? response?.statusText ?? 'Unknown error';
    return new Error(message);
  }


  // --- path utils

  private _basename(path: string): string {
    path = this._rtrim(path, '/');
    if (!path) {
      return '';
    }

    return path.substr(path.lastIndexOf('/') + 1);
  }

  private _dirname(path: string): string {
    path = this._rtrim(path, '/');
    if (!path) {
      return '/';
    }

    return path.substr(0, path.lastIndexOf('/'));
  }

  private _rtrim(haystack: string, needle: string): string {
    if (!haystack || !needle) {
      return haystack;
    }

    const needleLen = needle.length,
      haystackLen = haystack.length;

    if (needleLen === 0 || haystackLen === 0) {
      return haystack;
    }

    let offset = haystackLen,
      idx = -1;

    while (true) {
      idx = haystack.lastIndexOf(needle, offset - 1);
      if (idx === -1 || idx + needleLen !== offset) {
        break;
      }
      if (idx === 0) {
        return '';
      }
      offset = idx;
    }

    return haystack.substring(0, offset);
  }
}