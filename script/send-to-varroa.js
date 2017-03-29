// ==UserScript==
// @name           varroa musica
// @namespace      varroa
// @description    Adds a VM link for each torrent, to send directly to varroa musica.
// @include        http*://*redacted.ch/*
// @version        9
// @date           2017-03
// @grant          GM_getValue
// @grant          GM_setValue
// @grant          GM_notification
// ==/UserScript==

// with some help from `xo --fix send_to_varroa.js`
/* global window document MutationObserver GM_notification GM_getValue GM_setValue */
/* eslint new-cap: "off" */

const linkregex = /torrents\.php\?action=download.*?id=(\d+).*?authkey=.*?torrent_pass=(?=([a-z0-9]+))\2(?!&)/i;
const divider = ' | ';

const settings = getSettings();
const settingsPage = window.location.href.match('user.php\\?action=edit&userid=');
const top10Page = window.location.href.match('top10.php');
const torrentPage = window.location.href.match('torrents.php$');
const torrentUserPage = window.location.href.match('torrents.php?(.*)&userid');
let obsElem;
let linkLabel = 'VM';
if (top10Page) {
	linkLabel = '[' + linkLabel + ']';
}
let isWebSocketConnected = false;
let vmStatusLI = null;
let sock;
const hello = {
	Message: 'hello',
	Status: 1
};

if (settings.token && settings.url && settings.port) {
	newSocket();
	const alltorrents = [];
	for (let i = 0; i < document.links.length; i++) {
		alltorrents.push(document.links[i]);
	}

	for (let i = 0; i < alltorrents.length; i++) {
		if (linkregex.exec(alltorrents[i])) {
			const id = RegExp.$1;
			createLink(alltorrents[i], id);
		}
	}

	MutationObserver = window.MutationObserver || window.WebKitMutationObserver; // eslint-disable-line no-global-assign
	const obs = new MutationObserver(mutations => {
		mutations.forEach(mutation => {
			mutation.addedNodes.forEach(node => {
				if (linkregex.exec(node.querySelector('a'))) {
					const id = RegExp.$1;
					createLink(node.querySelector('a'), id);
				}
			});
		});
	});

	if (torrentPage) {
		obsElem = document.querySelector('#torrent_table > tbody'); // eslint-disable-line no-unused-vars
	} else if (torrentUserPage) {
		obsElem = document.querySelector('.torrent_table > tbody'); // eslint-disable-line no-unused-vars
	}
	if (obsElem) { // eslint-disable-line no-undef
		obs.observe(obsElem, { // eslint-disable-line no-undef
			childList: true
		});
	}
}

if (settingsPage) {
	appendSettings();
	document.getElementById('varroa_settings_token').addEventListener('change', saveSettings, false);
	document.getElementById('varroa_settings_url').addEventListener('change', saveSettings, false);
	document.getElementById('varroa_settings_port').addEventListener('change', saveSettings, false);
}

if (!settings && !settingsPage) {
	GM_notification({
		text: 'Missing configuration\nVisit user settings and setup',
		title: 'Varroa Musica:',
		timeout: 6000
	});
}

function newSocket() {
	// TODO use settings.url && settings.token
	sock = new WebSocket('ws://localhost:' + settings.port + '/ws');
	sock.onopen = function () {
		console.log('Connected to the server');
		isWebSocketConnected = true;
		// Send the msg object as a JSON-formatted string.
		sock.send(JSON.stringify(hello));
	};
	sock.onerror = function (evt) {
		console.log('Websocket error.');
		isWebSocketConnected = false;
		setVMStatus('VM KO');
	};
	sock.onmessage = function (evt) {
		console.log(evt.data);
		const msg = JSON.parse(evt.data);
		if (msg.Message === 'hello') {
			setVMStatus('VM OK');
		}
	};
	sock.onclose = function () {
		console.log('Server connection closed.');
		isWebSocketConnected = false;
		setVMStatus('VM KO');
	};
}

function setVMStatus(label) {
	const a = document.createElement('a');
	a.innerHTML = label;
	a.addEventListener('click', newSocket, false);
	if (vmStatusLI === null) {
		const target = document.getElementById('userinfo_stats');
		vmStatusLI = document.createElement('li');
		vmStatusLI.id = 'nav_varroa';
		vmStatusLI.appendChild(a);
		target.appendChild(vmStatusLI);
	} else {
		vmStatusLI.replaceChild(a, vmStatusLI.firstChild);
	}
}

function createLink(linkelement, id) {
	const link = document.createElement('varroa');
	link.appendChild(document.createElement('a'));
	link.firstChild.appendChild(document.createTextNode(linkLabel));
	link.appendChild(document.createTextNode(divider));
	link.firstChild.href = settings.url + ':' + settings.port + '/get/' + id + '?token=' + settings.token;
	link.firstChild.target = '_blank';
	link.firstChild.title = 'Direct download to varroa musica';
	linkelement.parentNode.insertBefore(link, linkelement);
}

function appendSettings() {
	const container = document.getElementsByClassName('main_column')[0];
	const lastTable = container.lastElementChild;
	let settingsHTML = '<a name="varroa_settings"></a>\n<table cellpadding="6" cellspacing="1" border="0" width="100%" class="layout border user_options" id="varroa_settings">\n';
	settingsHTML += '<tbody>\n<tr class="colhead_dark"><td colspan="2"><strong>Varroa Musica Settings (autosaved)</strong></td></tr>\n';
	settingsHTML += '<tr><td class="label" title="Token set in varroa">Token</td><td><input type="text" id="varroa_settings_token" placeholder="insert_your_token" value="' + GM_getValue('token', '') + '"></td></tr>\n';
	settingsHTML += '<tr><td class="label" title="Your seedbox hostname set in varroa">Hostname</td><td><input type="text" id="varroa_settings_url" placeholder="http://hostname.com" value="' + GM_getValue('url', '') + '"></td></tr>\n';
	settingsHTML += '<tr><td class="label" title="Your seedbox port set in varroa">Port</td><td><input type="text" id="varroa_settings_port" placeholder="your_chosen_port" value="' + GM_getValue('port', '') + '"></td></tr>\n';
	settingsHTML += '</tbody>\n</table>';
	lastTable.insertAdjacentHTML('afterend', settingsHTML);

	const sectionsElem = document.querySelectorAll('#settings_sections > ul')[0];
	const sectionsHTML = '<h2><a href="#varroa_settings" class="tooltip" title="Varroa Musica Settings">Varroa Musica</a></h2>';
	const li = document.createElement('li');
	li.innerHTML = sectionsHTML;
	sectionsElem.insertBefore(li, document.querySelectorAll('#settings_sections > ul > li:nth-child(10)')[0]);
}

function getSettings() {
	const token = GM_getValue('token', '');
	const url = GM_getValue('url', '');
	const port = GM_getValue('port', '');
	if (token && url && port) {
		return {
			token,
			url,
			port
		};
	}
	return false;
}

function saveSettings() {
	const elem = document.getElementById(this.id);
	const setting = this.id.replace('varroa_settings_', '');
	const border = elem.style.border;
	GM_setValue(setting, elem.value);
	if (GM_getValue(setting) === elem.value) {
		elem.style.border = '1px solid green';
		setTimeout(() => {
			elem.style.border = border;
		}, 2000);
	} else {
		elem.style.border = '1px solid red';
	}
}
