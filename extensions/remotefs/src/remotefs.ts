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
    this.baseUrl = `http://localhost:3000/api/v1/fs`;
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
      responseType: 'text',
      validateStatus: () => true
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    const data = JSON.parse(response.data);
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
    const response = await axios.get(url, { responseType: 'text', validateStatus: () => true });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
    const data: any[] = JSON.parse(response.data);
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
      // Create -> POST (target parent directory)
      const parent = parts.join('/');
      const url = `${this.baseUrl}/${parent}`;
      const form = new FormData();
      form.append('file', new Blob([buffer]), fileName);
      const response = await axios.post(url, form, {
        params: { overwrite: options.overwrite },
        validateStatus: () => true,
      });
      if (response.status < 200 || response.status >= 300) {
        throw this.mapError(response);
      }
      return this._fireSoon({ type: FileChangeType.Created, uri });
    }

    // Overwrite -> PUT (target parent directory)
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.put(url, buffer, {
      headers: { 'Content-Type': 'application/octet-stream' },
      params: { overwrite: options.overwrite },
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
    const newName = newUri.path.split('/').pop()!;
    const url = `${this.baseUrl}/${oldPath}`;
    const response = await axios.patch(url, { new_name: newName }, {
      headers: { 'Content-Type': 'application/json' },
      validateStatus: () => true,
    });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
  }

  async delete(uri: Uri): Promise<void> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.delete(url, { validateStatus: () => true });
    if (response.status < 200 || response.status >= 300) {
      throw this.mapError(response);
    }
  }

  async createDirectory(uri: Uri): Promise<void> {
    const path = uri.path.substring(1);
    const url = `${this.baseUrl}/${path}`;
    const response = await axios.post(url, undefined, {
      params: { create: true, overwrite: false },
      validateStatus: () => true
    });
    if (response.status < 200 || response.status >= 300) {
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
    const code = String(data?.code ?? '');

    switch (code) {
      case 'FILE_EXISTS':
        return FileSystemError.FileExists();
      case 'FILE_NOT_FOUND':
        return FileSystemError.FileNotFound();
      case 'FILE_IS_DIRECTORY':
        return FileSystemError.FileIsADirectory();
    }

    if (response?.status === 500) {
      return new Error('Internal server error');
    }

    const message = data?.error ?? response?.statusText ?? 'Unknown error';
    return new Error(message);
  }
}