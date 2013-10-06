/*
Copyright 2011 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

/**
 * @fileoverview Permanode description page.
 *
 */
goog.provide('camlistore.PermanodePage');

goog.require('goog.dom');
goog.require('goog.events.EventHandler');
goog.require('goog.events.EventType');
goog.require('goog.events.FileDropHandler');
goog.require('goog.ui.Component');
goog.require('camlistore.BlobItem');
goog.require('camlistore.BlobItemContainer');
goog.require('camlistore.ServerConnection');

/**
 * @param {camlistore.ServerType.DiscoveryDocument} config Global config
 *   of the current server this page is being rendered for.
 * @param {goog.dom.DomHelper=} opt_domHelper DOM helper to use.
 *
 * @extends {goog.ui.Component}
 * @constructor
 */
camlistore.PermanodePage = function(config, opt_domHelper) {
	goog.base(this, opt_domHelper);

	/**
	 * @type {Object}
	 * @private
	 */
	this.config_ = config;

	/**
	 * For members, not content.
	 * @type {camlistore.BlobItemContainer}
	 * @private
	 */
	this.blobItemContainer_ = new camlistore.BlobItemContainer(
		this.connection_, opt_domHelper
	);
	this.blobItemContainer_.thumbnailSize_ = camlistore.BlobItemContainer.THUMBNAIL_SIZES_[3];

	/**
	 * @type {camlistore.ServerType.DescribeResponse}
	 * @private
	 */
	this.describeResponse_ = null;

	/**
	 * @type {camlistore.ServerConnection}
	 * @private
	 */
	this.connection_ = new camlistore.ServerConnection(config);
};
goog.inherits(camlistore.PermanodePage, goog.ui.Component);


/**
 * Decorates an existing HTML DIV element.
 * @param {Element} element The DIV element to decorate.
 */
camlistore.PermanodePage.prototype.decorateInternal = function(element) {
	camlistore.PermanodePage.superClass_.decorateInternal.call(this, element);

	var el = this.getElement();
	goog.dom.classes.add(el, 'cam-permanode-page');

};


/** @override */
camlistore.PermanodePage.prototype.disposeInternal = function() {
	camlistore.PermanodePage.superClass_.disposeInternal.call(this);
	this.eh_.dispose();
};


/**
 * Called when component's element is known to be in the document.
 */
camlistore.PermanodePage.prototype.enterDocument = function() {
	camlistore.PermanodePage.superClass_.enterDocument.call(this);
	var permanode = getPermanodeParam();
	if (permanode) {
		goog.dom.getElement('permanode').innerHTML = "<a href='./?p=" + permanode + "'>" + permanode + "</a>";
		goog.dom.getElement('permanodeBlob').innerHTML = "<a href='./?b=" + permanode + "'>view blob</a>";
	}

	// TODO(mpl): use this.eh_ instead?
	// set up listeners
	goog.events.listen(goog.dom.getElement('formTitle'),
		goog.events.EventType.SUBMIT,
		this.handleFormTitleSubmit_,
		false, this);
	goog.events.listen(goog.dom.getElement('formTags'),
		goog.events.EventType.SUBMIT,
		this.handleFormTagsSubmit_,
		false, this);
	goog.events.listen(goog.dom.getElement('formAccess'),
		goog.events.EventType.SUBMIT,
		this.handleFormAccessSubmit_,
		false, this);
	goog.events.listen(goog.dom.getElement('btnGallery'),
		goog.events.EventType.CLICK,
		function() {
			var btnGallery = goog.dom.getElement('btnGallery');
			if (btnGallery.value == "list") {
				goog.dom.setTextContent(btnGallery, "List");
				btnGallery.value = "thumbnails";
			} else {
				goog.dom.setTextContent(btnGallery, "Thumbnails");
				btnGallery.value = "list";
			}
			this.reloadMembers_();
		},
		false, this);

	// set publish roots
	this.setupRootsDropdown_();

	// set dnd and form for file upload
	this.setupFilesHandlers_();

	this.describeBlob_()

	this.buildPathsList_()

	this.blobItemContainer_.render(goog.dom.getElement('membersThumbs'));
};

// Gets the |p| query parameter, assuming that it looks like a blobref.
function getPermanodeParam() {
	var blobRef = getQueryParam('p');
	return (blobRef && isPlausibleBlobRef(blobRef)) ? blobRef : null;
};

/**
 * Called when component's element is known to have been removed from the
 * document.
 */
camlistore.PermanodePage.prototype.exitDocument = function() {
	camlistore.PermanodePage.superClass_.exitDocument.call(this);
};

/**
 * @param {string} blobRef BlobRef for the uploaded file.
 * @param {string} permanode Permanode this blobRef is now the content of.
 * @private
 */
camlistore.PermanodePage.prototype.describeBlob_ = function() {
	var permanode = getPermanodeParam();
	this.connection_.describeWithThumbnails(
		permanode,
		this.blobItemContainer_.thumbnailSize_,
		goog.bind(this.handleDescribeBlob_, this, permanode),
		function(msg) {
			alert("failed to get blob description: " + msg);
		}
	);
};

/**
 * @param {string} permanode Node to describe.
 * @param {Object} describeResult Object of properties for the node.
 * @private
 */
camlistore.PermanodePage.prototype.handleDescribeBlob_ =
function(permanode, describeResult) {
	var meta = describeResult.meta;
	if (!meta[permanode]) {
		alert("didn't get blob " + permanode);
		return;
	}
	var permObj = meta[permanode].permanode;
	if (!permObj) {
		alert("blob " + permanode + " isn't a permanode");
		return;
	}
	this.describeResponse_ = describeResult;

	// title form
	var permTitleValue = permAttr(permObj, "title") ? permAttr(permObj, "title") : "";
	var inputTitle = goog.dom.getElement("inputTitle");
	inputTitle.value = permTitleValue;
	inputTitle.disabled = false;
	var btnSaveTitle = goog.dom.getElement("btnSaveTitle");
	btnSaveTitle.disabled = false;

	// tags form
	this.reloadTags_(permanode, describeResult);
	var inputNewTag = goog.dom.getElement("inputNewTag");
	inputNewTag.disabled = false;
	var btnAddTag = goog.dom.getElement("btnAddTag");
	btnAddTag.disabled = false;

	// access form
	var selectAccess = goog.dom.getElement("selectAccess");
	var accessValue = permAttr(permObj,"camliAccess") ? permAttr(permObj,"camliAccess") : "private";
	selectAccess.value = accessValue;
	selectAccess.disabled = false;
	var btnSaveAccess = goog.dom.getElement("btnSaveAccess");
	btnSaveAccess.disabled = false;

	// handle type detection
	handleType(permObj);

	// TODO(mpl): add a line showing something like
	// "Content: file (blobref)" or
	// "Content: directory (blobref)" or
	// "Content: None (has members)".

	// members
	this.reloadMembers_();

	// TODO(mpl): use a permanent blobItemContainer instead?
	/* blob content */
	var camliContent = permObj.attr.camliContent;
	if (camliContent && camliContent.length > 0) {
		var content = goog.dom.getElement('content');
		content.innerHTML = '';
		var useFileBlobrefAsLink = "true";
		var blobItem = new camlistore.BlobItem(permanode, meta, useFileBlobrefAsLink);
		blobItem.decorate(content);
		blobItem.setSize(300, 300);
		// TODO(mpl): ideally this should be done by handleType, but it's easier
		// to do it now that we have a blobItem object to work with.
		var isdir = blobItem.getDirBlobref_()
		var mountTip = goog.dom.getElement("cammountTip");
		goog.dom.removeChildren(mountTip);
		if (isdir != "") {
			var tip = "Mount with:";
			goog.dom.setTextContent(mountTip, tip);
			goog.dom.appendChild(mountTip, goog.dom.createDom("br"));
			var codeTip = goog.dom.createDom("code");
			goog.dom.setTextContent(codeTip, "$ cammount /some/mountpoint " + isdir);
			goog.dom.appendChild(mountTip, codeTip);
		}
	}

	// debug attrs
	goog.dom.setTextContent(goog.dom.getElement("debugattrs"), JSON.stringify(permObj.attr, null, 2));

};

// TODO(mpl): pass directly the permanode object
/**
 * @param {string} permanode Node to describe.
 * @param {Object} describeResult Object of properties for the node.
 * @private
 */
camlistore.PermanodePage.prototype.reloadTags_ = function(permanode, describeResult) {
	var permanodeObject = describeResult.meta[permanode].permanode;
	var spanTags = document.getElementById("spanTags");
	while (spanTags.firstChild) {
		spanTags.removeChild(spanTags.firstChild);
	}
	var tags = permanodeObject.attr.tag;
	for (idx in tags) {
		var tag = tags[idx];

		var tagSpan = goog.dom.createDom("span");
		tagSpan.className = 'cam-permanode-tag-c';
		var tagTextEl = goog.dom.createDom("span");
		tagTextEl.className = 'cam-permanode-tag';
		goog.dom.setTextContent(tagTextEl, tag);
		goog.dom.appendChild(tagSpan, tagTextEl);

		var tagDel = goog.dom.createDom("span");
		tagDel.className = 'cam-permanode-del';
		goog.dom.setTextContent(tagDel, "x");
		goog.events.listen(tagDel,
			goog.events.EventType.CLICK,
			this.deleteTagFunc_(tag, tagTextEl, tagSpan),
			false, this
		);

		goog.dom.appendChild(tagSpan, tagDel);
		goog.dom.appendChild(spanTags, tagSpan);
	}
}

/**
 * @param {Object} tag tag value to remove.
 * @param {Object} strikeEle text element to strike while we wait for the removal to take effect.
 * @param {Object} removeEle element to remove.
 * @return {Function}
 * @private
 */
camlistore.PermanodePage.prototype.deleteTagFunc_ =
function(tag, strikeEle, removeEle) {
	var delFunc = function(e) {
		strikeEle.innerHTML = "<del>" + strikeEle.innerHTML + "</del>";
		this.connection_.newDelAttributeClaim(
			getPermanodeParam(),
			"tag",
			tag,
			function() {
				removeEle.parentNode.removeChild(removeEle);
			},
			function(msg) {
				alert(msg);
			}
		);
	};
	return goog.bind(delFunc, this);
}


/**
 * @private
 */
camlistore.PermanodePage.prototype.reloadMembers_ =
function() {
	var meta = this.describeResponse_.meta;
	var permanode = getPermanodeParam();
	var members = meta[permanode].permanode.attr.camliMember;
	var membersList = goog.dom.getElement('membersList');
	var membersThumbs = goog.dom.getElement('membersThumbs');
	membersList.innerHTML = '';
	if (members && members.length > 0) {
		var btnGallery = goog.dom.getElement('btnGallery');
		var doThumbnails = (btnGallery.value == "thumbnails");
		for (idx in members) {
			var member = members[idx];
			this.addMember_(member, meta, doThumbnails);
		}
		if (doThumbnails) {
			this.blobItemContainer_.show_();
		} else {
			this.blobItemContainer_.hide_();
			this.blobItemContainer_.resetChildren_();
		}
	}
}


/**
 * @param {string} pn child permanode.
 * @param {Object} meta meta in describe response.
 * @param {booleon} thumbnails whether to display thumbnails or a list
 * @private
 */
camlistore.PermanodePage.prototype.addMember_ =
function(pn, meta, thumbnails) {
	var blobItem = new camlistore.BlobItem(pn, meta);
	if (thumbnails) {
		this.blobItemContainer_.addChild(blobItem, true)
	} else {
		var membersList = goog.dom.getElement("membersList");
		var ul;
		if (membersList.innerHTML == "") {
			ul = goog.dom.createDom("ul");
			goog.dom.appendChild(membersList, ul);
		} else {
			ul = membersList.firstChild;
		}
		var li = goog.dom.createDom("li");
		var a = goog.dom.createDom("a");
		a.href = "./?p=" + pn;
		goog.dom.setTextContent(a, blobItem.getTitle_());

		var del = goog.dom.createDom("span");
		del.className = 'cam-permanode-del';
		goog.dom.setTextContent(del, "x");
		goog.events.listen(del,
			goog.events.EventType.CLICK,
			this.deleteMemberFunc_(pn, a, li),
			false, this
		);

		goog.dom.appendChild(li, a);
		goog.dom.appendChild(li, del);
		goog.dom.appendChild(ul, li);
	}
}

/**
 * @param {string} member child permanode
 * @param {Object} strikeEle text element to strike while we wait for the removal to take effect.
 * @param {Object} removeEle element to remove.
 * @return {Function}
 * @private
 */
camlistore.PermanodePage.prototype.deleteMemberFunc_ =
function(member, strikeEle, removeEle) {
	var delFunc = function(e) {
		strikeEle.innerHTML = "<del>" + strikeEle.innerHTML + "</del>";
		this.connection_.newDelAttributeClaim(
			getPermanodeParam(),
			"camliMember",
			member,
			goog.bind(function() {
				removeEle.parentNode.removeChild(removeEle);
				// TODO(mpl): refreshing the whole thing is kindof heavy, maybe?
				this.describeBlob_();
			}, this),
			function(msg) {
				alert(msg);
			}
		);
	};
	return goog.bind(delFunc, this);
}

/**
 * @param {string} sourcePermanode permanode pointed by the path.
 * @param {string} path path to remove.
 * @param {Object} strikeEle element to remove.
 * @return {Function}
 * @private
 */
camlistore.PermanodePage.prototype.deletePathFunc_ =
function(sourcePermanode, path, strikeEle) {
	var delFunc = function(e) {
		strikeEle.innerHTML = "<del>" + strikeEle.innerHTML + "</del>";
		this.connection_.newDelAttributeClaim(
			sourcePermanode,
			"camliPath:" + path,
			getPermanodeParam(),
			goog.bind(function() {
				this.buildPathsList_();
			}, this),
			function(msg) {
				alert(msg);
			}
		);
	};
	return goog.bind(delFunc, this);
}

/**
 * @param {goog.events.Event} e The title form submit event.
 * @private
 */
camlistore.PermanodePage.prototype.handleFormTitleSubmit_ = function(e) {
	e.stopPropagation();
	e.preventDefault();

	var inputTitle = goog.dom.getElement("inputTitle");
	inputTitle.disabled = true;
	var btnSaveTitle = goog.dom.getElement("btnSaveTitle");
	btnSaveTitle.disabled = true;

	var startTime = new Date();
	this.connection_.newSetAttributeClaim(
		getPermanodeParam(), "title", inputTitle.value,
		goog.bind(function() {
			var elapsedMs = new Date().getTime() - startTime.getTime();
			setTimeout(goog.bind(function() {
				inputTitle.disabled = false;
				btnSaveTitle.disabled = false;
				this.describeBlob_();
			},this), Math.max(250 - elapsedMs, 0));
		}, this),
		function(msg) {
			alert(msg);
			inputTitle.disabled = false;
			btnSaveTitle.disabled = false;
		}
	);
};

/**
 * @param {goog.events.Event} e The tags form submit event.
 * @private
 */
camlistore.PermanodePage.prototype.handleFormTagsSubmit_ = function(e) {
	e.stopPropagation();
	e.preventDefault();

	var input = goog.dom.getElement("inputNewTag");
	var btn = goog.dom.getElement("btnAddTag");
	if (input.value == "") {
		return;
	}
	input.disabled = true;
	btn.disabled = true;

	var startTime = new Date();
	var tags = input.value.split(/\s*,\s*/);
	var nRemain = tags.length;
	var oneDone = goog.bind(function() {
		nRemain--;
		if (nRemain == 0) {
			var elapsedMs = new Date().getTime() - startTime.getTime();
			setTimeout(goog.bind(function() {
				input.value = '';
				input.disabled = false;
				btn.disabled = false;
				this.describeBlob_();
			}, this), Math.max(250 - elapsedMs, 0));
		}
	}, this);
	for (idx in tags) {
		var tag = tags[idx];
		this.connection_.newAddAttributeClaim(
			getPermanodeParam(), "tag", tag, oneDone,
			function(msg) {
				alert(msg);
				oneDone();
			}
		);
	}
};

/**
 * @param {goog.events.Event} e The access form submit event.
 * @private
 */
camlistore.PermanodePage.prototype.handleFormAccessSubmit_ = function(e) {
	e.stopPropagation();
	e.preventDefault();

	var selectAccess = goog.dom.getElement("selectAccess");
	selectAccess.disabled = true;
	var btnSaveAccess = goog.dom.getElement("btnSaveAccess");
	btnSaveAccess.disabled = true;

	var operation = this.connection_.newDelAttributeClaim;
	var value = "";
	if (selectAccess.value != "private") {
		operation = this.connection_.newSetAttributeClaim;
		value = selectAccess.value;
	}

	var startTime = new Date();
	operation = goog.bind(operation, this.connection_);
	operation(
		getPermanodeParam(),
		"camliAccess",
		value,
		function() {
			var elapsedMs = new Date().getTime() - startTime.getTime();
			setTimeout(function() {
				selectAccess.disabled = false;
				btnSaveAccess.disabled = false;
			}, Math.max(250 - elapsedMs, 0));
		},
		function(msg) {
			alert(msg);
			selectAccess.disabled = false;
			btnSaveAccess.disabled = false;
		}
	);

};

/**
 * @private
 */
camlistore.PermanodePage.prototype.setupRootsDropdown_ =
function() {
	var selRoots = goog.dom.getElement("selectPublishRoot");
	if (!this.config_.publishRoots) {
		console.log("no publish roots");
		return;
	}
	for (var rootName in this.config_.publishRoots) {
		var opt = goog.dom.createElement("option");
		opt.setAttribute("value", rootName);
		goog.dom.appendChild(opt,
			goog.dom.createTextNode(this.config_.publishRoots[rootName].prefix[0])
		);
		goog.dom.appendChild(selRoots, opt);
	}
	goog.events.listen(goog.dom.getElement("btnSavePublish"),
		goog.events.EventType.CLICK,
		this.handleSavePublish_,
		false, this
	);
};


/**
 * @param {goog.events.Event} e The access form submit event.
 * @private
 */
camlistore.PermanodePage.prototype.handleSavePublish_ =
function(e) {
	var selRoots = goog.dom.getElement("selectPublishRoot");
	var suffix = goog.dom.getElement("publishSuffix");

	var ourPermanode = getPermanodeParam();
	if (!ourPermanode) {
		return;
	}

	var publishRoot = selRoots.value;
	if (!publishRoot) {
		alert("no publish root selected");
		return;
	}
	var pathSuffix = suffix.value;
	if (!pathSuffix) {
		alert("no path suffix specified");
		return;
	}

	selRoots.disabled = true;
	suffix.disabled = true;

	var enabled = function() {
		selRoots.disabled = false;
		suffix.disabled = false;
	};

	// Step 1: resolve selRoots.value -> blobref of the root's permanode.
	// Step 2: set attribute on the root's permanode, or a sub-permanode
	// if multiple path components in suffix:
	// "camliPath:<suffix>" => permanode-of-ourselves

	var sigconf = this.config_.signing;
	var handleFindCamliRoot = function(pnres) {
		if (!pnres.permanode) {
			alert("failed to publish root's permanode");
			enabled();
			return;
		}
		var handleSetCamliPath = function() {
			console.log("success.");
			enabled();
			selRoots.value = "";
			suffix.value = "";
			this.buildPathsList_();
		};
		var handleFailCamliPath = function() {
			alert("failed to set attribute");
			enabled();
		};
		this.connection_.newSetAttributeClaim(
			pnres.permanode, "camliPath:" + pathSuffix, ourPermanode,
			goog.bind(handleSetCamliPath, this), handleFailCamliPath
		);
	};
	var handleFailFindCamliRoot = function() {
		alert("failed to find publish root's permanode");
		enabled();
	};
	this.connection_.permanodeOfSignerAttrValue(
		sigconf.publicKeyBlobRef, "camliRoot", publishRoot,
		goog.bind(handleFindCamliRoot, this), handleFailFindCamliRoot
	);
};

/**
 * @private
 */
camlistore.PermanodePage.prototype.buildPathsList_ =
function() {
	var ourPermanode = getPermanodeParam();
	if (!ourPermanode) {
		return;
	}
	var sigconf = this.config_.signing;

	var handleFindPath = function(jres) {
		var div = goog.dom.getElement("existingPaths");

		// TODO: there can be multiple paths in this list, but the HTML
		// UI only shows one.  The UI should show all, and when adding a new one
		// prompt users whether they want to add to or replace the existing one.
		// For now we just update the UI to show one.
		// alert(JSON.stringify(jres, null, 2));
		if (jres.paths && jres.paths.length > 0) {
			div.innerHTML = "Existing paths for this permanode:";
			var ul = goog.dom.createElement("ul");
			goog.dom.appendChild(div,ul);
			for (var idx in jres.paths) {
				var path = jres.paths[idx];
				var li = goog.dom.createElement("li");
				var span = goog.dom.createElement("span");
				goog.dom.appendChild(li,span);

				var blobLink = goog.dom.createElement("a");
				blobLink.href = ".?p=" + path.baseRef;
				goog.dom.setTextContent(blobLink, path.baseRef);
				goog.dom.appendChild(span,blobLink);

				goog.dom.appendChild(span,goog.dom.createTextNode(" - "));

				var pathLink = goog.dom.createElement("a");
				pathLink.href = "";
				goog.dom.setTextContent(pathLink, path.suffix);
				for (var key in this.config_.publishRoots) {
					var root = this.config_.publishRoots[key];
					if (root.currentPermanode == path.baseRef) {
						// Prefix should include a trailing slash.
						pathLink.href = root.prefix[0] + path.suffix;
						// TODO: Check if we're the latest permanode
						// for this path and display some "old" notice
						// if not.
						break;
					}
				}
				goog.dom.appendChild(span,pathLink);

				var del = goog.dom.createElement("span");
				del.className = "cam-permanode-del";
				goog.dom.setTextContent(del, "x");
				goog.events.listen(del,
					goog.events.EventType.CLICK,
					this.deletePathFunc_(path.baseRef, path.suffix, span),
					false, this
				);
				goog.dom.appendChild(span,del);

				goog.dom.appendChild(ul,li);
			}
		} else {
			div.innerHTML = "";
		}
	};
	this.connection_.pathsOfSignerTarget(sigconf.publicKeyBlobRef, ourPermanode,
		goog.bind(handleFindPath, this), alert
	);
};

// TODO(mpl): reuse blobitem code for dnd?
/**
 * @private
 */
camlistore.PermanodePage.prototype.setupFilesHandlers_ =
function() {
	var dnd = goog.dom.getElement("dnd");
	goog.events.listen(goog.dom.getElement("fileForm"),
		goog.events.EventType.SUBMIT,
		this.handleFilesSubmit_,
		false, this
	);
	goog.events.listen(goog.dom.getElement("fileInput"),
		goog.events.EventType.CHANGE,
		onFileInputChange,
		false, this
	);

	var stop = function(e) {
		this.classList &&
			goog.dom.classes.add(this, 'cam-permanode-dnd-over');
		e.stopPropagation();
		e.preventDefault();
	};
	goog.events.listen(dnd,
		goog.events.EventType.DRAGENTER,
		stop, false, this
	);
	goog.events.listen(dnd,
		goog.events.EventType.DRAGOVER,
		stop, false, this
	);
	goog.events.listen(dnd,
		goog.events.EventType.DRAGLEAVE,
		goog.bind(function() {
			goog.dom.classes.remove(this, 'cam-permanode-dnd-over');
		}, this), false, this
	);

	var drop = function(e) {
		goog.dom.classes.remove(this, 'cam-permanode-dnd-over');
		stop(e);
		var dt = e.getBrowserEvent().dataTransfer;
		var files = dt.files;
		goog.dom.getElement("info").innerHTML = "";
		this.handleFiles_(files);
	};
	goog.events.listen(dnd,
		goog.events.FileDropHandler.EventType.DROP,
		goog.bind(drop, this),
		false, this
	);
}


/**
 * @param {goog.events.Event} e The title form submit event.
 * @private
 */
camlistore.PermanodePage.prototype.handleFilesSubmit_ =
function(e) {
	e.stopPropagation();
	e.preventDefault();
	this.handleFiles_(document.getElementById("fileInput").files);
}


/**
 * @param {Array<files>} files the files to upload.
 * @private
 */
camlistore.PermanodePage.prototype.handleFiles_ =
function(files) {
	for (var i = 0; i < files.length; i++) {
		var file = files[i];
		this.startFileUpload_(file);
	}
}

/**
 * @param {file} file the file to upload.
 * @private
 */
camlistore.PermanodePage.prototype.startFileUpload_ =
function(file) {
	var dnd = goog.dom.getElement("dnd");
	var up = goog.dom.createElement("div");
	up.className= 'cam-permanode-dnd-item';
	goog.dom.appendChild(dnd, up);
	var info = "name=" + file.name + " size=" + file.size + "; type=" + file.type;

	var setStatus = function(status) {
		up.innerHTML = info + " " + status;
	};
	setStatus("(scanning)");

	var onFail = function(msg) {
		up.innerHTML = info + " <strong>fail:</strong> ";
		goog.dom.appendChild(up, goog.dom.createTextNode(msg));
	};

	var onGotFileSchemaRef = function(fileref) {
		setStatus(" <strong>fileref: " + fileref + "</strong>");
		this.connection_.createPermanode(
			goog.bind(function(filepn) {
				var doneWithAll = goog.bind(function() {
					setStatus("- done");
					this.describeBlob_();
				}, this);
				var addMemberToParent = function() {
					setStatus("adding member");
					this.connection_.newAddAttributeClaim(
						getPermanodeParam(), "camliMember", filepn,
						doneWithAll, onFail
					);
				};
				var makePermanode = goog.bind(function() {
					setStatus("making permanode");
					this.connection_.newSetAttributeClaim(
						filepn, "camliContent", fileref,
						goog.bind(addMemberToParent, this), onFail
					);
				}, this);
				makePermanode();
			}, this),
			onFail
		);
	};

	this.connection_.uploadFile(file,
		goog.bind(onGotFileSchemaRef, this),
		onFail,
		function(contentsRef) {
			setStatus("(checking for dup of " + contentsRef + ")");
		}
	);
}

// Returns the first value from the query string corresponding to |key|.
// Returns null if the key isn't present.
getQueryParam = function(key) {
	var params = document.location.search.substring(1).split('&');
	for (var i = 0; i < params.length; ++i) {
		var parts = params[i].split('=');
		if (parts.length == 2 && decodeURIComponent(parts[0]) == key)
			return decodeURIComponent(parts[1]);
	}
	return null;
};

// Returns true if the passed-in string might be a blobref.
isPlausibleBlobRef = function(blobRef) {
	return /^\w+-[a-f0-9]+$/.test(blobRef);
};

function permAttr(permanodeObject, name) {
	if (!(name in permanodeObject.attr)) {
		return null;
	}
	if (permanodeObject.attr[name].length == 0) {
		return null;
	}
	return permanodeObject.attr[name][0];
};

function handleType(permObj) {
	var disablePublish = false;
	var selType = goog.dom.getElement("type");
	var dnd = goog.dom.getElement("dnd");
	var btnGallery = goog.dom.getElement("btnGallery");
	var membersDiv = goog.dom.getElement("members");
	dnd.style.display = "none";
	btnGallery.style.visibility = 'hidden';
	goog.dom.setTextContent(membersDiv, "");
	if (permAttr(permObj, "camliRoot")) {
		disablePublish = true;  // can't give a URL to a root with a claim
	} else if (permAttr(permObj, "camliMember")) {
		dnd.style.display = "block";
		btnGallery.style.visibility = 'visible';
		goog.dom.setTextContent(membersDiv, "Members:");
	}

	goog.dom.getElement("selectPublishRoot").disabled = disablePublish;
	goog.dom.getElement("publishSuffix").disabled = disablePublish;
	goog.dom.getElement("btnSavePublish").disabled = disablePublish;
};


function $(id) { return goog.dom.getElement(id) }

function onFileInputChange(e) {
    var s = "";
    var files = $("fileInput").files;
    for (var i = 0; i < files.length; i++) {
        var file = files[i];
        s += "<p>" + file.name + "</p>";
    }
    var fl = $("filelist");
    fl.innerHTML = s;
}

