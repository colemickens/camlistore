/**
 * @fileoverview Contains a set of BlobItems. Knows how to fetch items from
 * the server side. Is preconfigured with common queries like "recent" blobs.
 *
 */
goog.provide('camlistore.BlobItemContainer');

goog.require('goog.dom');
goog.require('goog.dom.classes');
goog.require('goog.events.Event');
goog.require('goog.events.EventHandler');
goog.require('goog.events.EventType');
goog.require('goog.events.FileDropHandler');
goog.require('goog.ui.Container');
goog.require('camlistore.BlobItem');
goog.require('camlistore.ServerConnection');


/**
 * @param {camlistore.ServerConnection} connection Connection to the server
 *   for fetching blobrefs and other queries.
 * @param {goog.dom.DomHelper=} opt_domHelper DOM helper to use.
 *
 * @extends {goog.ui.Container}
 * @constructor
 */
camlistore.BlobItemContainer = function(connection, opt_domHelper) {
  goog.base(this, opt_domHelper);

  /**
   * @type {Array.<camlistore.BlobItem>}
   * @private
   */
  this.checkedBlobItems_ = [];

  /**
   * @type {camlistore.ServerConnection}
   * @private
   */
  this.connection_ = connection;

  /**
   * BlobRef of the permanode defined as the current collection/set.
   * Selected blobitems will be added as members of that collection
   * upon relevant actions (e.g click on the 'Add to Set' toolbar button).
   * @type {string}
   * @private
   */
  this.currentCollec_ = "";

  /**
   * Whether our content has changed since last layout.
   * @type {boolean}
   * @private
   */
  this.isLayoutDirty_ = false;

  /**
   * @type {goog.events.EventHandler}
   * @private
   */
  this.eh_ = new goog.events.EventHandler(this);

  /**
    * An id for a timer we use to know when the drag has ended.
    * @type {number}
    * @private
    */
  this.dragEndTimer_ = 0;

  /**
   * Whether the blobItems within can be selected.
   * @type {boolean}
   */
  this.isSelectionEnabled = false;

  /**
   * Whether users can drag files onto the container to upload.
   * @type {boolean}
   */
  this.isFileDragEnabled = false;

  this.setFocusable(false);
};
goog.inherits(camlistore.BlobItemContainer, goog.ui.Container);


/**
 * Margin between items in the layout.
 * @type {number}
 */
camlistore.BlobItemContainer.BLOB_ITEM_MARGIN = 7;

/**
 * If the last row uses at least this much of the available width before
 * adjustments, we'll call it "close enough" and adjust things so that it fills
 * the entire row. Less than this, and we'll leave the last row unaligned.
 */
camlistore.BlobItemContainer.LAST_ROW_CLOSE_ENOUGH_TO_FULL = 0.85;


/**
 * @type {Array.<number>}
 */
camlistore.BlobItemContainer.THUMBNAIL_SIZES_ = [75, 100, 150, 200, 250];


/**
 * @type {goog.events.FileDropHandler}
 * @private
 */
camlistore.BlobItemContainer.prototype.fileDropHandler_ = null;


/**
 * @type {Element}
 * @private
 */
camlistore.BlobItemContainer.prototype.dragActiveElement_ = null;


/**
 * Constants for events fired by BlobItemContainer
 * @enum {string}
 */
camlistore.BlobItemContainer.EventType = {
  BLOB_ITEMS_CHOSEN: 'Camlistore_BlobItemContainer_BlobItems_Chosen',
  SINGLE_NODE_CHOSEN: 'Camlistore_BlobItemContainer_SingleNode_Chosen'
};


/**
 * @type {number}
 * @private
 */
camlistore.BlobItemContainer.prototype.thumbnailSize_ = 200;

/**
 * @return {boolean}
 */
camlistore.BlobItemContainer.prototype.smaller = function() {
  var index = camlistore.BlobItemContainer.THUMBNAIL_SIZES_.indexOf(
      this.thumbnailSize_);
  if (index == 0) {
    return false;
  }
  var el = this.getElement();
  goog.dom.classes.remove(el, 'cam-blobitemcontainer-' + this.thumbnailSize_);
  this.thumbnailSize_ = camlistore.BlobItemContainer.THUMBNAIL_SIZES_[index-1];
  goog.dom.classes.add(el, 'cam-blobitemcontainer-' + this.thumbnailSize_);
  return true;
};


/**
 * @return {boolean}
 */
camlistore.BlobItemContainer.prototype.bigger = function() {
  var index = camlistore.BlobItemContainer.THUMBNAIL_SIZES_.indexOf(
      this.thumbnailSize_);
  if (index == camlistore.BlobItemContainer.THUMBNAIL_SIZES_.length - 1) {
    return false;
  }
  var el = this.getElement();
  goog.dom.classes.remove(el, 'cam-blobitemcontainer-' + this.thumbnailSize_);
  this.thumbnailSize_ = camlistore.BlobItemContainer.THUMBNAIL_SIZES_[index+1];
  goog.dom.classes.add(el, 'cam-blobitemcontainer-' + this.thumbnailSize_);
  return true;
};


/**
 * Creates an initial DOM representation for the component.
 */
camlistore.BlobItemContainer.prototype.createDom = function() {
  this.decorateInternal(this.dom_.createElement('div'));
};


/**
 * Decorates an existing HTML DIV element.
 * @param {Element} element The DIV element to decorate.
 */
camlistore.BlobItemContainer.prototype.decorateInternal = function(element) {
  camlistore.BlobItemContainer.superClass_.decorateInternal.call(this, element);
  this.layout_();

  var el = this.getElement();
  goog.dom.classes.add(el, 'cam-blobitemcontainer');
  goog.dom.classes.add(el, 'cam-blobitemcontainer-' + this.thumbnailSize_);
};


/** @override */
camlistore.BlobItemContainer.prototype.disposeInternal = function() {
  camlistore.BlobItemContainer.superClass_.disposeInternal.call(this);
  this.eh_.dispose();
};


/** @override */
camlistore.BlobItemContainer.prototype.addChildAt =
function(child, index, opt_render) {
  goog.base(this, "addChildAt", child, index, opt_render);
  child.setEnabled(this.isSelectionEnabled);
  if (!this.isLayoutDirty_) {
    var raf = window.requestAnimationFrame || window.mozRequestAnimationFrame ||
      window.webkitRequestAnimationFrame || window.msRequestAnimationFrame;
    // It's OK if raf not supported, the timer loop we have going will pick up
    // the layout a little later.
    if (raf) {
      raf(goog.bind(function() {
        this.layout_(false);
      }, this, false));
    }

    this.isLayoutDirty_ = true;
  }
};


/**
 * Called when component's element is known to be in the document.
 */
camlistore.BlobItemContainer.prototype.enterDocument = function() {
  camlistore.BlobItemContainer.superClass_.enterDocument.call(this);

  this.resetChildren_();
  this.listenToBlobItemEvents_();

  if (this.isFileDragEnabled) {
    this.fileDragListener_ = goog.bind(this.handleFileDrag_, this);
    this.eh_.listen(document,
                    goog.events.EventType.DRAGOVER,
                    this.fileDragListener_);
    this.eh_.listen(document,
                    goog.events.EventType.DRAGENTER,
                    this.fileDragListener_);

    this.fileDropHandler_ = new goog.events.FileDropHandler(document);
    this.registerDisposable(this.fileDropHandler_);
    this.eh_.listen(
      this.fileDropHandler_,
      goog.events.FileDropHandler.EventType.DROP,
      this.handleFileDrop_);
  }

  // We can't catch everything that could cause us to need to relayout. Instead,
  // be lazy and just poll every second.
  window.setInterval(goog.bind(function() {
    this.layout_(false);
  }, this, false), 1000);
};


/**
 * Called when component's element is known to have been removed from the
 * document.
 */
camlistore.BlobItemContainer.prototype.exitDocument = function() {
  camlistore.BlobItemContainer.superClass_.exitDocument.call(this);
  this.eh_.removeAll();
};


/**
 * Show recent blobs.
 */
camlistore.BlobItemContainer.prototype.showRecent = function() {
  this.connection_.getRecentlyUpdatedPermanodes(
      goog.bind(this.showRecentDone_, this),
      this.thumbnailSize_);
};


/**
 * Show roots
 */
camlistore.BlobItemContainer.prototype.showRoots =
function(sigconf) {
	this.connection_.permanodesWithAttr(sigconf.publicKeyBlobRef,
		"camliRoot", "", false, 0, this.thumbnailSize_,
		goog.bind(this.showWithAttrDone_, this),
		function(msg) {
			alert(msg);
		}
	);
};

/**
 * Search and show permanodes matching the specified criteria.
 * @param {string} sigconf
 * @param {string} attr
 * @param {string} value
 * @param {boolean} fuzzy Noop because not supported yet.
 * @param {number} max max number of items in response.
 */
camlistore.BlobItemContainer.prototype.showWithAttr =
function(sigconf, attr, value, fuzzy, max) {
	this.connection_.permanodesWithAttr(sigconf.publicKeyBlobRef,
		attr, value, fuzzy, max, this.thumbnailSize_,
		goog.bind(this.showWithAttrDone_, this),
		function(msg) {
			alert(msg);
		}
	);
};

/**
 * Search for a permanode with the required blobref
 * @param {string} blobref
 */
camlistore.BlobItemContainer.prototype.findByBlobref_ =
function(blobref) {
	this.connection_.describeWithThumbnails(
		blobref,
		this.thumbnailSize_,
		goog.bind(this.findByBlobrefDone_, this, blobref),
		function(msg) {
			alert(msg);
		}
	);
};

/**
 * @return {Array.<camlistore.BlobItem>}
 */
camlistore.BlobItemContainer.prototype.getCheckedBlobItems = function() {
  return this.checkedBlobItems_;
};


/**
 * Subscribes to events dispatched by blob items.
 * @private
 */
camlistore.BlobItemContainer.prototype.listenToBlobItemEvents_ = function() {
  var doc = goog.dom.getOwnerDocument(this.element_);
  this.eh_.
      listen(this, goog.ui.Component.EventType.CHECK,
             this.handleBlobItemChecked_).
      listen(this, goog.ui.Component.EventType.UNCHECK,
             this.handleBlobItemChecked_).
      listen(doc,
             goog.events.EventType.KEYDOWN,
             this.handleKeyDownEvent_).
      listen(doc,
             goog.events.EventType.KEYUP,
             this.handleKeyUpEvent_);
};


/**
 * @type {boolean}
 * @private
 */
camlistore.BlobItemContainer.prototype.isShiftKeyDown_ = false;


/**
 * @type {boolean}
 * @private
 */
camlistore.BlobItemContainer.prototype.isCtrlKeyDown_ = false;


/**
 * Sets state for whether or not the shift or ctrl key is down.
 * @param {goog.events.KeyEvent} e A key event.
 */
camlistore.BlobItemContainer.prototype.handleKeyDownEvent_ = function(e) {
  if (e.keyCode == goog.events.KeyCodes.SHIFT) {
    this.isShiftKeyDown_ = true;
    this.isCtrlKeyDown_ = false;
    return;
  }
  if (e.keyCode == goog.events.KeyCodes.CTRL) {
    this.isCtrlKeyDown_ = true;
    this.isShiftKeyDown_ = false;
    return;
  }
};


/**
 * Sets state for whether or not the shift or ctrl key is up.
 * @param {goog.events.KeyEvent} e A key event.
 */
camlistore.BlobItemContainer.prototype.handleKeyUpEvent_ = function(e) {
  this.isShiftKeyDown_ = false;
  this.isCtrlKeyDown_ = false;
};


/**
 * @param {goog.events.Event} e An event.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleBlobItemChecked_ = function(e) {
  // Because the CHECK/UNCHECK event dispatches before isChecked is set.
  // We stop the default behaviour because want to control manually here whether
  // the source blobitem gets checked or not. See http://camlistore.org/issue/134
  e.preventDefault();
  var blobItem = e.target;
  var isCheckingItem = !blobItem.isChecked();
  var isShiftMultiSelect = this.isShiftKeyDown_;
  var isCtrlMultiSelect = this.isCtrlKeyDown_;

  if (isShiftMultiSelect || isCtrlMultiSelect) {
    var lastChildSelected =
        this.checkedBlobItems_[this.checkedBlobItems_.length - 1];
    var firstChildSelected =
        this.checkedBlobItems_[0];
    var lastChosenIndex = this.indexOfChild(lastChildSelected);
    var firstChosenIndex = this.indexOfChild(firstChildSelected);
    var thisIndex = this.indexOfChild(blobItem);
  }

  if (isShiftMultiSelect) {
    // deselect all items after the chosen one
    for (var i = lastChosenIndex; i > thisIndex; i--) {
      var item = this.getChildAt(i);
      item.setState(goog.ui.Component.State.CHECKED, false);
      if (goog.array.contains(this.checkedBlobItems_, item)) {
        goog.array.remove(this.checkedBlobItems_, item);
      }
    }
    // make sure all the others are selected.
    for (var i = firstChosenIndex; i <= thisIndex; i++) {
      var item = this.getChildAt(i);
      item.setState(goog.ui.Component.State.CHECKED, true);
      if (!goog.array.contains(this.checkedBlobItems_, item)) {
        this.checkedBlobItems_.push(item);
      }
    }
    this.dispatchEvent(camlistore.BlobItemContainer.EventType.BLOB_ITEMS_CHOSEN);
  } else if (isCtrlMultiSelect) {
    if (isCheckingItem) {
      blobItem.setState(goog.ui.Component.State.CHECKED, true);
      if (!goog.array.contains(this.checkedBlobItems_, blobItem)) {
        var pos = -1;
        for (var i = 0; i <= this.checkedBlobItems_.length; i++) {
          var idx = this.indexOfChild(this.checkedBlobItems_[i]);
          if (idx > thisIndex) {
            pos = i;
            break;
          }
        }
        if (pos != -1) {
          goog.array.insertAt(this.checkedBlobItems_, blobItem, pos)
        } else {
          this.checkedBlobItems_.push(blobItem);
        }
      }
    } else {
      blobItem.setState(goog.ui.Component.State.CHECKED, false);
      if (goog.array.contains(this.checkedBlobItems_, blobItem)) {
        var done = goog.array.remove(this.checkedBlobItems_, blobItem);
        if (!done) {
          alert("Failed to remove item from selection");
        }
      }
    }
    this.dispatchEvent(camlistore.BlobItemContainer.EventType.BLOB_ITEMS_CHOSEN);
  } else {
    blobItem.setState(goog.ui.Component.State.CHECKED, isCheckingItem);
    if (isCheckingItem) {
      this.checkedBlobItems_.push(blobItem);
    } else {
      goog.array.remove(this.checkedBlobItems_, blobItem);
    }
    this.dispatchEvent(camlistore.BlobItemContainer.EventType.SINGLE_NODE_CHOSEN);
  }
};

/**
 */
camlistore.BlobItemContainer.prototype.unselectAll =
function() {
	goog.array.forEach(this.checkedBlobItems_, function(item) {
		item.setState(goog.ui.Component.State.CHECKED, false);
	});
	this.checkedBlobItems_ = [];
}

/**
 * @param {camlistore.ServerType.IndexerMetaBag} result JSON response to this request.
 * @private
 */
camlistore.BlobItemContainer.prototype.showRecentDone_ = function(result) {
  this.resetChildren_();
  if (!result || !result.recent) {
    return;
  }

  for (var i = 0, n = result.recent.length; i < n; i++) {
    var blobRef = result.recent[i].blobref;
    var item = new camlistore.BlobItem(blobRef, result.meta);
    this.addChild(item, true);
  }
};

/**
 * @param {bool} Whether to force layout.
 * @private
 */
camlistore.BlobItemContainer.prototype.layout_ = function(force) {
  var el = this.getElement();
  var availWidth = el.clientWidth;

  if (!force && !this.isLayoutDirty_ && availWidth == this.lastClientWidth_) {
    return;
  }

  this.isLayoutDirty_ = false;
  this.lastClientWidth_ = availWidth;

  var currentTop = this.constructor.BLOB_ITEM_MARGIN;
  var currentWidth = this.constructor.BLOB_ITEM_MARGIN;
  var rowStart = 0;
  var lastItem = this.getChildCount() - 1;

  for (var i = rowStart; i <= lastItem; i++) {
    var item = this.getChildAt(i);

    var nextWidth = currentWidth + this.thumbnailSize_ * item.getThumbAspect() +
      this.constructor.BLOB_ITEM_MARGIN;
    if (i != lastItem && nextWidth < availWidth) {
      currentWidth = nextWidth;
      continue;
    }

    var rowEnd, rowWidth;
    if (i == lastItem) {
      rowEnd = lastItem;
      rowWidth = nextWidth;
      if (nextWidth / availWidth <
          this.constructor.LAST_ROW_CLOSE_ENOUGH_TO_FULL) {
        availWidth = nextWidth;
      }
    } else if (availWidth - currentWidth <= nextWidth - availWidth) {
      rowEnd = i - 1;
      rowWidth = currentWidth;
    } else {
      rowEnd = i;
      rowWidth = nextWidth;
    }

    currentTop += this.layoutRow_(rowStart, rowEnd, availWidth, rowWidth,
                                  currentTop) +
      this.constructor.BLOB_ITEM_MARGIN;

    currentWidth = this.constructor.BLOB_ITEM_MARGIN;
    rowStart = rowEnd + 1;
    i = rowEnd;
  }

  el.style.height = currentTop + this.constructor.BLOB_ITEM_MARGIN + 'px';
};

/**
 * @param {Number} startIndex The index of the first item in the row.
 * @param {Number} endIndex The index of the last item in the row.
 * @param {Number} availWidth The width available to the row for layout.
 * @param {Number} usedWidth The width that the contents of the row consume
 * using their initial dimensions, before any scaling or clipping.
 * @param {Number} top The position of the top of the row.
 * @return {Number} The height of the row after layout.
 * @private
 */
camlistore.BlobItemContainer.prototype.layoutRow_ =
function(startIndex, endIndex, availWidth, usedWidth, top) {
  var currentLeft = 0;
  var rowHeight = Number.POSITIVE_INFINITY;

  var numItems = endIndex - startIndex + 1;
  var availThumbWidth = availWidth -
    (this.constructor.BLOB_ITEM_MARGIN * (numItems + 1));
  var usedThumbWidth = usedWidth -
    (this.constructor.BLOB_ITEM_MARGIN * (numItems + 1));

  for (var i = startIndex; i <= endIndex; i++) {
    var item = this.getChildAt(i);

    // We figure out the amount to adjust each item in this slightly non-
    // intuitive way so that the adjustment is split up as fairly as possible.
    // Figuring out a ratio up front and applying it to all items uniformly can
    // end up with a large amount left over because of rounding.
    var numItemsLeft = (endIndex + 1) - i;
    var delta = Math.round((availThumbWidth - usedThumbWidth) / numItemsLeft);
    var originalWidth = this.thumbnailSize_ * item.getThumbAspect();
    var width = originalWidth + delta;
    var ratio = width / originalWidth;
    var height = Math.round(this.thumbnailSize_ * ratio);

    var elm = item.getElement();
    elm.style.position = 'absolute';
    elm.style.left = currentLeft + this.constructor.BLOB_ITEM_MARGIN + 'px';
    elm.style.top = top + 'px';
    item.setSize(width, height);

    currentLeft += width + this.constructor.BLOB_ITEM_MARGIN;
    usedThumbWidth += delta;
    rowHeight = Math.min(rowHeight, height);
  }

  for (var i = startIndex; i <= endIndex; i++) {
    this.getChildAt(i).setHeight(rowHeight);
  }

  return rowHeight;
};

/**
 * @param {camlistore.ServerType.SearchWithAttrResponse} result JSON response to
 * this request.
 * @private
 */
camlistore.BlobItemContainer.prototype.showWithAttrDone_ = function(result) {
  this.resetChildren_();
  if (!result) {
    return;
  }
  var results = result.withAttr;
  var meta = result.meta;
  if (!results || !meta) {
    return;
  }

  for (var i = 0, n = results.length; i < n; i++) {
    var blobRef = results[i].permanode;
    var item = new camlistore.BlobItem(blobRef, meta);
    this.addChild(item, true);
  }
};

/**
 * @param {camlistore.ServerType.DescribeResponse} result JSON response to this request.
 * @private
 */
camlistore.BlobItemContainer.prototype.findByBlobrefDone_ =
function(permanode, result) {
	this.resetChildren_();
	if (!result) {
		return;
	}
	var meta = result.meta;
	if (!meta || !meta[permanode]) {
		return;
	}
	var item = new camlistore.BlobItem(permanode, meta);
	this.addChild(item, true);
};

/**
 * Clears all children from this container, reseting to the default state.
 */
camlistore.BlobItemContainer.prototype.resetChildren_ = function() {
  this.removeChildren(true);
};


/**
 * @param {goog.events.Event} e The drag drop event.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleFileDrop_ = function(e) {
  var recipient = this.dragActiveElement_;
  if (!recipient) {
    console.log("No valid target to drag and drop on.");
    return;
  }

  goog.dom.classes.remove(recipient.getElement(), 'cam-dropactive');
  this.dragActiveElement_ = null;

  var files = e.getBrowserEvent().dataTransfer.files;
  for (var i = 0, n = files.length; i < n; i++) {
    var file = files[i];
    // TODO(bslatkin): Add an uploading item placeholder while the upload
    // is in progress. Somehow pipe through the POST progress.
    this.connection_.uploadFile(
      file, goog.bind(this.handleUploadSuccess_, this, file, recipient.blobRef_));
  }
};


/**
 * @param {File} file File to upload.
 * @param {string} blobRef BlobRef for the uploaded file.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleUploadSuccess_ =
    function(file, recipient, blobRef) {
  this.connection_.createPermanode(
      goog.bind(this.handleCreatePermanodeSuccess_, this, file, recipient, blobRef));
};


/**
 * @param {File} file File to upload.
 * @param {string} blobRef BlobRef for the uploaded file.
 * @param {string} permanode Permanode this blobRef is now the content of.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleCreatePermanodeSuccess_ =
    function(file, recipient, blobRef, permanode) {
  this.connection_.newSetAttributeClaim(
      permanode, 'camliContent', blobRef,
      goog.bind(this.handleSetAttributeSuccess_, this,
                file, recipient, blobRef, permanode));
};


/**
 * @param {File} file File to upload.
 * @param {string} blobRef BlobRef for the uploaded file.
 * @param {string} permanode Permanode this blobRef is now the content of.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleSetAttributeSuccess_ =
function(file, recipient, blobRef, permanode) {
	this.connection_.describeWithThumbnails(
		permanode,
		this.thumbnailSize_,
		goog.bind(this.handleDescribeSuccess_, this, recipient, permanode));
};


/**
 * @param {string} permanode Node to describe.
 * @param {camlistore.ServerType.DescribeResponse} describeResult Object of properties for the node.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleDescribeSuccess_ =
  function(recipient, permanode, describeResult) {
	var item = new camlistore.BlobItem(permanode, describeResult.meta);
	this.addChildAt(item, 0, true);
	if (!recipient) {
		return;
	}
	this.connection_.newAddAttributeClaim(
		recipient, 'camliMember', permanode);
};

/**
 * @param {goog.events.Event} e The drag enter event.
 * @private
 */
camlistore.BlobItemContainer.prototype.handleFileDrag_ = function(e) {
  if (this.dragEndTimer_) {
    this.dragEndTimer_ = window.clearTimeout(this.dragEndTimer_);
  }
  this.dragEndTimer_ = window.setTimeout(this.fileDragListener_, 2000);

  var activeElement = e ? this.getOwnerControl(e.target) : e;
  if (activeElement) {
    if (!activeElement.isCollection()) {
      activeElement = this;
    }
  } else if (e) {
    activeElement = this;
  }

  if (activeElement == this.dragActiveElement_) {
    return;
  }

  if (this.dragActiveElement_) {
    goog.dom.classes.remove(this.dragActiveElement_.getElement(),
                            'cam-dropactive');
  }

  this.dragActiveElement_ = activeElement;

  if (this.dragActiveElement_) {
    goog.dom.classes.add(this.dragActiveElement_.getElement(),
                         'cam-dropactive');
  }
};

/**
 * @private
 */
camlistore.BlobItemContainer.prototype.hide_ = function() {
  goog.dom.classes.remove(this.getElement(),
                          'cam-blobitemcontainer-' + this.thumbnailSize_);
  goog.dom.classes.add(this.getElement(),
                       'cam-blobitemcontainer-hidden');
};

/**
 * @private
 */
camlistore.BlobItemContainer.prototype.show_ = function() {
  goog.dom.classes.remove(this.getElement(),
                          'cam-blobitemcontainer-hidden');
  goog.dom.classes.add(this.getElement(),
                       'cam-blobitemcontainer-' + this.thumbnailSize_);
  this.layout_(true);
};
