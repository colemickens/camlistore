/*
Copyright 2013 Google Inc.

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

goog.provide('cam.DetailView');

goog.require('goog.array');
goog.require('goog.events.EventHandler');
goog.require('goog.math.Size');
goog.require('goog.object');
goog.require('goog.string');

goog.require('cam.AnimationLoop');
goog.require('cam.BlobItemReactData');
goog.require('cam.imageUtil');
goog.require('cam.Navigator');
goog.require('cam.reactUtil');
goog.require('cam.SearchSession');
goog.require('cam.SpritedAnimation');

cam.DetailView = React.createClass({
	displayName: 'DetailView',

	IMG_MARGIN: 20,
	PIGGY_WIDTH: 88,
	PIGGY_HEIGHT: 62,

	propTypes: {
		blobref: React.PropTypes.string.isRequired,
		getDetailURL: React.PropTypes.func.isRequired,
		history: cam.reactUtil.quacksLike({go:React.PropTypes.func.isRequired}).isRequired,
		height: React.PropTypes.number.isRequired,
		keyEventTarget: React.PropTypes.object.isRequired, // An event target we will addEventListener() on to receive key events.
		navigator: React.PropTypes.instanceOf(cam.Navigator).isRequired,
		oldURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		searchSession: React.PropTypes.instanceOf(cam.SearchSession).isRequired,
		searchURL: React.PropTypes.instanceOf(goog.Uri).isRequired,
		width: React.PropTypes.number.isRequired,
	},

	getInitialState: function() {
		this.imgSize_ = null;
		this.lastImageHeight_ = 0;
		this.pendingNavigation_ = 0;
		this.navCount_ = 1;
		this.eh_ = new goog.events.EventHandler(this);

		return {
			imgHasLoaded: false,
			backwardPiggy: false,
		};
	},

	componentWillReceiveProps: function(nextProps) {
		if (this.props.blobref != nextProps.blobref) {
			this.blobItemData_ = null;
			this.imgSize_ = null;
			this.lastImageHeight_ = 0;
			this.setState({imgHasLoaded: false});
		}
	},

	componentDidMount: function(root) {
		this.eh_.listen(this.props.searchSession, cam.SearchSession.SEARCH_SESSION_CHANGED, this.searchUpdated_);
		this.eh_.listen(this.props.keyEventTarget, 'keyup', this.handleKeyUp_);
		this.searchUpdated_();
	},

	componentDidUpdate: function(prevProps, prevState) {
		var img = this.getImageRef_();
		if (img) {
			// This function gets called multiple times, but the DOM de-dupes listeners for us. Thanks DOM.
			img.getDOMNode().addEventListener('load', this.onImgLoad_);
			img.getDOMNode().addEventListener('error', function() {
				console.error('Could not load image: %s', img.props.src);
			})
		}
	},

	render: function() {
		this.blobItemData_ = this.getBlobItemData_();
		this.imgSize_ = this.getImgSize_();
		return (
			React.DOM.div({className:'detail-view', style: this.getStyle_()},
				this.getImg_(),
				this.getPiggy_(),
				React.DOM.div({className:'detail-view-sidebar', key:'sidebar', style: this.getSidebarStyle_()},
					React.DOM.a({key:'search-link', href:this.props.searchURL.toString(), onClick:this.handleEscape_}, 'Back to search'),
					' - ',
					React.DOM.a({key:'old-link', href:this.props.oldURL.toString()}, 'Old and busted'),
					React.DOM.pre({key:'sidebar-pre'}, JSON.stringify(this.getPermanodeMeta_(), null, 2)))));
	},

	componentWillUnmount: function() {
		this.eh_.dispose();
	},

	handleKeyUp_: function(e) {
		if (e.keyCode == goog.events.KeyCodes.LEFT) {
			this.navigate_(-1);
		} else if (e.keyCode == goog.events.KeyCodes.RIGHT) {
			this.navigate_(1);
		} else if (e.keyCode == goog.events.KeyCodes.ESC) {
			this.handleEscape_(e);
		}
	},

	navigate_: function(offset) {
		this.pendingNavigation_ = offset;
		++this.navCount_;
		this.setState({backwardPiggy: offset < 0});
		this.handlePendingNavigation_();
	},

	handleEscape_: function(e) {
		e.preventDefault();
		e.stopPropagation();
		history.go(-this.navCount_);
	},

	handlePendingNavigation_: function() {
		if (!this.pendingNavigation_) {
			return;
		}

		var results = this.props.searchSession.getCurrentResults();
		var index = goog.array.findIndex(results.blobs, function(elm) {
			return elm.blob == this.props.blobref;
		}.bind(this));

		if (index == -1) {
			this.props.searchSession.loadMoreResults();
			return;
		}

		index += this.pendingNavigation_;
		if (index < 0) {
			this.pendingNavigation_ = 0;
			console.log('Cannot navigate past beginning of search result.');
			return;
		}

		if (index >= results.blobs.length) {
			if (this.props.searchSession.isComplete()) {
				this.pendingNavigation_ = 0;
				console.log('Cannot navigate past end of search result.');
			} else {
				this.props.searchSession.loadMoreResults();
			}
			return;
		}

		this.props.navigator.navigate(this.props.getDetailURL(results.blobs[index].blob));
	},

	onImgLoad_: function() {
		this.setState({imgHasLoaded:true});
	},

	searchUpdated_: function() {
		this.handlePendingNavigation_();

		this.blobItemData_ = this.getBlobItemData_();
		if (this.blobItemData_) {
			this.forceUpdate();
			return;
		}

		if (this.props.searchSession.isComplete()) {
			// TODO(aa): 404 UI.
			var error = goog.string.subs('Could not find blobref %s in search session.', this.props.blobref);
			alert(error);
			throw new Error(error);
		}

		// TODO(aa): This can be inefficient in the case of a fresh page load if we have to load lots of pages to find the blobref.
		// Our search protocol needs to be updated to handle the case of paging ahead to a particular item.
		this.props.searchSession.loadMoreResults();
	},

	getImg_: function() {
		var transition = React.addons.TransitionGroup({transitionName: 'detail-img'}, []);
		if (this.imgSize_) {
			transition.props.children.push(
				React.DOM.img({
					className: React.addons.classSet({
						'detail-view-img': true,
						'detail-view-img-loaded': this.state.imgHasLoaded
					}),
					// We want each image to have its own node in the DOM so that during the crossfade, we don't see the image jump to the next image's size.
					key: this.getImageId_(),
					ref: this.getImageId_(),
					src: this.getSrc_(),
					style: this.getCenteredProps_(this.imgSize_.width, this.imgSize_.height)
				})
			);
		}
		return transition;
	},

	getPiggy_: function() {
		var transition = React.addons.TransitionGroup({transitionName: 'detail-piggy'}, []);
		if (!this.state.imgHasLoaded) {
			transition.props.children.push(
				cam.SpritedAnimation({
					src: 'glitch/npc_piggy__x1_walk_png_1354829432.png',
					className: React.addons.classSet({
						'detail-view-piggy': true,
						'detail-view-piggy-backward': this.state.backwardPiggy
					}),
					spriteWidth: this.PIGGY_WIDTH,
					spriteHeight: this.PIGGY_HEIGHT,
					sheetWidth: 8,
					sheetHeight: 3,
					interval: 30,
					style: this.getCenteredProps_(this.PIGGY_WIDTH, this.PIGGY_HEIGHT)
				}));
		}
		return transition;
	},

	getCenteredProps_: function(w, h) {
		var avail = new goog.math.Size(this.props.width - this.getSidebarWidth_(), this.props.height);
		return {
			top: (avail.height - h) / 2,
			left: (avail.width - w) / 2,
			width: w,
			height: h
		}
	},

	getSrc_: function() {
		this.lastImageHeight_ = Math.min(this.blobItemData_.im.height, cam.imageUtil.getSizeToRequest(this.imgSize_.height, this.lastImageHeight_));
		var uri = new goog.Uri(this.blobItemData_.m.thumbnailSrc);
		uri.setParameterValue('mh', this.lastImageHeight_);
		return uri.toString();
	},

	getImgSize_: function() {
		if (!this.blobItemData_) {
			return null;
		}
		var rawSize = new goog.math.Size(this.blobItemData_.im.width, this.blobItemData_.im.height);
		var available = new goog.math.Size(
			this.props.width - this.getSidebarWidth_() - this.IMG_MARGIN * 2,
			this.props.height - this.IMG_MARGIN * 2);
		if (rawSize.height <= available.height && rawSize.width <= available.width) {
			return rawSize;
		}
		return rawSize.scaleToFit(available);
	},

	getStyle_: function() {
		return {
			width: this.props.width,
			height: this.props.height
		}
	},

	getSidebarStyle_: function() {
		return {
			width: this.getSidebarWidth_()
		}
	},

	getSidebarWidth_: function() {
		return Math.max(this.props.width * 0.2, 300);
	},

	getPermanodeMeta_: function() {
		if (!this.blobItemData_) {
			return null;
		}
		return this.blobItemData_.m;
	},

	getBlobItemData_: function() {
		var metabag = this.props.searchSession.getCurrentResults().description.meta;
		if (!metabag[this.props.blobref]) {
			return null;
		}
		return new cam.BlobItemReactData(this.props.blobref, metabag);
	},

	getImageRef_: function() {
		return this.refs[this.getImageId_()];
	},

	getImageId_: function() {
		return 'img' + this.props.blobref;
	}
});
