// ==UserScript==
// @name           varroa musica
// @namespace      varroa
// @description    Adds a VM link for each torrent, to send directly to varroa musica.
// @include        http*://*redacted.ch/*
// @version        1
// @date           2017-03.27
// ==/UserScript==

// EDIT THIS
var weblink = "https://hostname:your_chosen_port/get/";
var token = "insert_your_token";

var linkregex = /torrents\.php\?action=download.*?id=(\d+).*?authkey=.*?torrent_pass=(?=([a-z0-9]+))\2(?!&)/i;
var divider = ' | ';

alltorrents = [];
for (var i=0; i < document.links.length; i++) {
    alltorrents.push(document.links[i]);
}

for (var i=0; i < alltorrents.length; i++) {
    if (linkregex.exec(alltorrents[i])) {
        id = RegExp.$1;
        createLink(alltorrents[i],id);
    }
}

function createLink(linkelement, id) {
    var link = document.createElement("varroa");
    link.appendChild(document.createElement("a"));
    link.firstChild.appendChild(document.createTextNode("VM"));
    link.appendChild(document.createTextNode(divider));
    link.firstChild.href=weblink+id+"?token="+token;

    link.firstChild.target="_blank";
    link.firstChild.title="Direct Download to varroa musica";
    linkelement.parentNode.insertBefore(link, linkelement);
}