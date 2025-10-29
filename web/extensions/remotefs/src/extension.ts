/*---------------------------------------------------------------------------------------------
 *  Copyright (c) Microsoft Corporation. All rights reserved.
 *  Licensed under the MIT License. See License.txt in the project root for license information.
 *--------------------------------------------------------------------------------------------*/

//
// ############################################################################
//
//						! USED FOR RUNNING VSCODE OUT OF SOURCES FOR WEB !
//										! DO NOT REMOVE !
//
// ############################################################################
//

import * as vscode from 'vscode';
import { RemoteFS } from './remotefs';
import { MemFS } from './memfs';

declare const navigator: unknown;

export function activate(context: vscode.ExtensionContext) {
	if (typeof navigator === 'object') {	// do not run under node.js
		const remoteFs = enableFs(context);
	}
  context.messagePassingProtocol?.postMessage({ type: "ready" });
}

function enableFs(context: vscode.ExtensionContext): RemoteFS {
	const remoteFs = new RemoteFS();
	context.subscriptions.push(remoteFs);

	return remoteFs;
}
