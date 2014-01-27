/*
Copyright 2013 The Camlistore Authors.

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

goog.provide('cam.GalleryPage');

goog.require('goog.dom');
goog.require('goog.events.EventHandler');
goog.require('goog.events.EventType');
goog.require('goog.ui.Component');

goog.require('cam.ServerConnection');

// @param {cam.ServerType.DiscoveryDocument} config Global config of the current server this page is being rendered for.
// @param {goog.dom.DomHelper=} opt_domHelper DOM helper to use.
cam.GalleryPage = function(config, opt_domHelper) {
	goog.base(this, opt_domHelper);

	this.config_ = config;
	this.connection_ = new cam.ServerConnection(config);
};
goog.inherits(cam.GalleryPage, goog.ui.Component);

cam.GalleryPage.prototype.decorateInternal = function(element) {
	cam.GalleryPage.superClass_.decorateInternal.call(this, element);
};

cam.GalleryPage.prototype.disposeInternal = function() {
	cam.GalleryPage.superClass_.disposeInternal.call(this);
};

cam.GalleryPage.prototype.enterDocument = function() {
	cam.GalleryPage.superClass_.enterDocument.call(this);

	var members = goog.dom.getElement('members');
	if (!members) {
		return;
	}
	var children = goog.dom.getChildren(members);
	if (!children || children.length < 1) {
		return;
	}
	goog.array.forEach(children, function(li) {
		li.src = li.src + '&square=1';
	})

	if (camliViewIsOwner) {
		var el = this.getElement();
		goog.dom.classes.add(el, 'camliadmin');

		goog.array.forEach(children, function(li) {
			var lichild = goog.dom.getFirstElementChild(li);
			var titleSpan = goog.dom.getLastElementChild(lichild);
			var editLink = goog.dom.createElement('a', {'href': '#'});
			goog.dom.classes.add(editLink, 'hidden');
			goog.dom.setTextContent(editLink, 'edit title');

			var titleInput = goog.dom.createElement('input');
			goog.dom.classes.add(titleInput, 'hidden');

			goog.events.listen(editLink,
				goog.events.EventType.CLICK,
				function(e) {
					goog.dom.classes.remove(titleSpan, 'visible');
					goog.dom.classes.add(titleSpan, 'hidden');
					goog.dom.classes.remove(titleInput, 'hidden');
					goog.dom.classes.add(titleInput, 'visible');
					titleInput.focus();
					titleInput.select();
					e.stopPropagation();
					e.preventDefault();
				},
				false, this
			);
			goog.events.listen(li,
				goog.events.EventType.MOUSEOVER,
					function(e) {
						goog.dom.classes.remove(editLink, 'hidden');
						goog.dom.classes.add(editLink, 'title-edit');
					},
					false, this
			);
			goog.events.listen(li,
				goog.events.EventType.MOUSEOUT,
					function(e) {
						goog.dom.classes.remove(editLink, 'title-edit');
						goog.dom.classes.add(editLink, 'hidden');
						goog.dom.classes.remove(titleInput, 'visible');
						goog.dom.classes.add(titleInput, 'hidden');
						goog.dom.classes.remove(titleSpan, 'hidden');
						goog.dom.classes.add(titleSpan, 'visible');
					},
					false, this
			);
			goog.events.listen(titleInput,
				goog.events.EventType.KEYPRESS,
				goog.bind(function(e) {
					if (e.keyCode == 13) {
						this.saveImgTitle_(titleInput, titleSpan);
					}
				}, this),
				false, this
			);
			goog.dom.insertSiblingBefore(editLink, titleSpan);
			goog.dom.insertChildAt(li, titleInput, 1);
			}, this
		)
	}
}

// @param {string} titleInput text field element for title
// @param {string} titleSpan span element containing the title
cam.GalleryPage.prototype.saveImgTitle_ = function (titleInput, titleSpan) {
	var spanText = goog.dom.getTextContent(titleSpan);
	var newVal = titleInput.value;
	if (newVal != "" && newVal != spanText) {
		goog.dom.setTextContent(titleSpan, newVal);
		var blobRef = goog.dom.getParentElement(titleInput).id.replace(/^camli-/, '');
		this.connection_.newSetAttributeClaim(
			blobRef,
			"title",
			newVal,
			function() {
			},
			function(msg) {
				alert(msg);
			}
		);
	}
	goog.dom.classes.remove(titleInput, 'visible');
	goog.dom.classes.add(titleInput, 'hidden');
	goog.dom.classes.remove(titleSpan, 'hidden');
	goog.dom.classes.add(titleSpan, 'visible');
}

cam.GalleryPage.prototype.exitDocument = function() {
	cam.GalleryPage.superClass_.exitDocument.call(this);
};
