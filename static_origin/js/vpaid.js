var getVPAIDAd = function () {
    return new VpaidVideoPlayer();
};

//<editor-fold desc="VapidVideoPlayer">
var VpaidVideoPlayer = function () {
    this.slot_ = null;
    this.videoSlot_ = null;
    this.isDebug = true;

    this.eventsCallbacks_ = {};
    this.attributes_ = {
        'companions': '',
        'desiredBitrate': 256,
        'duration': 10,
        'expanded': false,
        'height': 0,
        'icons': '',
        'linear': true,
        'remainingTime': 10,
        'skippableState': false,
        'viewMode': 'normal',
        'width': 0,
        'volume': 1.0
    };
    this.quartileEvents_ = [
        {event: 'AdVideoStart', value: 0},
        {event: 'AdVideoFirstQuartile', value: 25},
        {event: 'AdVideoMidpoint', value: 50},
        {event: 'AdVideoThirdQuartile', value: 75},
        {event: 'AdVideoComplete', value: 100}
    ];
    this.nextQuartileIndex_ = 0;
    this.parameters_ = {};

    this.adIndex = 0;
    this.adTagUrl_ = null;
    this.adTagRequestUrl_ = null;
    this.adTagImpressionUrl_ = null;
};

VpaidVideoPlayer.prototype.handshakeVersion = function (version) {
    return (version);
};

VpaidVideoPlayer.prototype.initAd = function (width, height, viewMode, desiredBitrate, creativeData, environmentVars) {
    //if (this.isDebug) console.clear();

    this.attributes_['width'] = width;
    this.attributes_['height'] = height;
    this.attributes_['viewMode'] = viewMode;
    this.attributes_['desiredBitrate'] = desiredBitrate;

    this.slot_ = environmentVars.slot;
    this.videoSlot_ = environmentVars.videoSlot;

    this.parameters_ = JSON.parse(creativeData['AdParameters']);

    this.initIMASdk_();

    // this.updateVideoSlot_();


    // this.videoSlot_.addEventListener('timeupdate', this.timeUpdateHandler_.bind(this), false);
    // this.videoSlot_.addEventListener('loadedmetadata', this.loadedMetadata_.bind(this), false);
    // this.videoSlot_.addEventListener('ended', this.stopAd.bind(this), false);
    // this.callEvent_('AdLoaded');
};

VpaidVideoPlayer.prototype.initIMASdk_ = function () {
    var this_ = this;
    var ima3 = document.createElement('script');
    ima3.setAttribute('src', 'https://imasdk.googleapis.com/js/sdkloader/ima3.js');
    ima3.onload = function () {
        this_.insertAd_();
    };
    this.slot_.appendChild(ima3);
};

VpaidVideoPlayer.prototype.insertAd_ = function () {
    var player = document.createElement('div');
    player.id = 'videoplayer';
    player.style.display = 'block';
    player.style.position = 'absolute';
    player.style.background = 'black';
    player.style.width = this.attributes_['width'] + "px";
    player.style.height = this.attributes_['height'] + "px";
    player.style.top = 0;
    player.style.left = 0;
    player.style.zIndex = 999;

    var content = document.createElement('video');
    content.id = 'content';

    var adcontainer = document.createElement('div');
    adcontainer.id = 'adcontainer';
    adcontainer.style.position = 'relative';

    player.appendChild(adcontainer);
    player.appendChild(content);

    this.slot_.appendChild(player);

    this.videoPlayer_ = new VideoPlayer(this.attributes_['width'], this.attributes_['height']);
    this.ads_ = new Ads(this, this.videoPlayer_);

    this.callEvent_('AdLoaded');
};

VpaidVideoPlayer.prototype.startWaterfall_ = function () {
    //TODO change 'url' for new param
    this.log(this.parameters_);
    if (this.parameters_[this.adIndex] != undefined) {
        this.log('calling ' + this.adIndex);
        this.adTagUrl_ = this.parameters_[this.adIndex].url;
        this.adTagRequestUrl_ = this.parameters_[this.adIndex].r;
        this.adTagImpressionUrl_ = this.parameters_[this.adIndex].i;
        this.ads_.initialUserAction();
        this.videoPlayer_.preloadContent(this.bind_(this, this.loadAds_));
    } else {
        this.onContentEnded_();
    }

    // if (this.adIndex < this.parameters_.tags.length) {
    //     this.adTagUrl_ = this.parameters_.tags[this.adIndex].url;
    //     this.ads_.initialUserAction();
    //     this.videoPlayer_.preloadContent(this.bind_(this, this.loadAds_));
    // } else {
    //     this.onContentEnded_();
    // }
};

// VpaidVideoPlayer.prototype.loadedMetadata_ = function () {
//     this.attributes_['duration'] = this.videoSlot_.duration;
//     this.callEvent_('AdDurationChange');
// };
//
// VpaidVideoPlayer.prototype.timeUpdateHandler_ = function () {
//     if (this.nextQuartileIndex_ >= this.quartileEvents_.length) {
//         return;
//     }
//     var percentPlayed = this.videoSlot_.currentTime * 100.0 / this.videoSlot_.duration;
//     if (percentPlayed >= this.quartileEvents_[this.nextQuartileIndex_].value) {
//         this.eventsCallbacks_[this.quartileEvents_[this.nextQuartileIndex_].event]();
//         this.nextQuartileIndex_ += 1;
//     }
//     if (this.videoSlot_.duration > 0) {
//         this.attributes_['remainingTime'] = this.videoSlot_.duration - this.videoSlot_.currentTime;
//     }
// };
//
// VpaidVideoPlayer.prototype.updateVideoSlot_ = function () {
//     if (this.videoSlot_ == null) {
//         this.videoSlot_ = document.createElement('video');
//         this.log('Warning: No video element passed to ad, creating element.');
//         this.slot_.appendChild(this.videoSlot_);
//     }
//     this.updateVideoPlayerSize_();
//     var foundSource = false;
//     var videos = this.parameters_.videos || [];
//     for (var i = 0; i < videos.length; i++) {
//         if (this.videoSlot_.canPlayType(videos[i].mimetype) != '') {
//             this.videoSlot_.setAttribute('src', videos[i].url);
//             foundSource = true;
//             break;
//         }
//     }
//     if (!foundSource) {
//         this.callEvent_('AdError');
//     }
// };

VpaidVideoPlayer.prototype.updateVideoPlayerSize_ = function () {
    this.ads_.resize(this.attributes_['width'], this.attributes_['height']);
};

VpaidVideoPlayer.prototype.startAd = function () {
    this.startWaterfall_();
    this.log('Starting ad');
    this.callEvent_('AdStarted');
};

VpaidVideoPlayer.prototype.stopAd = function () {
    this.log('Stopping ad');
    var callback = this.callEvent_.bind(this);
    setTimeout(callback, 75, ['AdStopped']);
};

VpaidVideoPlayer.prototype.resizeAd = function (width, height, viewMode) {
    this.log('resizeAd ' + width + 'x' + height + ' ' + viewMode);
    this.attributes_['width'] = width;
    this.attributes_['height'] = height;
    this.attributes_['viewMode'] = viewMode;

    //TODO change function of resize
    this.updateVideoPlayerSize_();
    this.callEvent_('AdSizeChange');
};

VpaidVideoPlayer.prototype.pauseAd = function () {
    this.ads_.pause();
};

VpaidVideoPlayer.prototype.resumeAd = function () {
    this.ads_.resume();
};

VpaidVideoPlayer.prototype.expandAd = function () {
    this.log('expandAd');
    this.attributes_['expanded'] = true;
    this.callEvent_('AdExpanded');
};

VpaidVideoPlayer.prototype.collapseAd = function () {
    this.log('collapseAd');
    this.attributes_['expanded'] = false;
};

VpaidVideoPlayer.prototype.skipAd = function () {
    //TODO CHECK if call
    this.log('skipAd');
    var skippableState = this.attributes_['skippableState'];
    if (skippableState) {
        this.callEvent_('AdSkipped');
    }
};

VpaidVideoPlayer.prototype.subscribe = function (aCallback, eventName, aContext) {
    this.log('Subscribe ' + eventName);
    this.eventsCallbacks_[eventName] = aCallback.bind(aContext);
};

VpaidVideoPlayer.prototype.unsubscribe = function (eventName) {
    this.log('unsubscribe ' + eventName);
    this.eventsCallbacks_[eventName] = null;
};

VpaidVideoPlayer.prototype.getAdLinear = function () {
    return this.attributes_['linear'];
};

VpaidVideoPlayer.prototype.getAdWidth = function () {
    return this.attributes_['width'];
};

VpaidVideoPlayer.prototype.getAdHeight = function () {
    return this.attributes_['height'];
};

VpaidVideoPlayer.prototype.getAdExpanded = function () {
    return this.attributes_['expanded'];
};

VpaidVideoPlayer.prototype.getAdSkippableState = function () {
    return this.attributes_['skippableState'];
};

VpaidVideoPlayer.prototype.getAdRemainingTime = function () {
    return this.attributes_['remainingTime'];
};

VpaidVideoPlayer.prototype.getAdDuration = function () {
    return this.attributes_['duration'];
};

VpaidVideoPlayer.prototype.getAdVolume = function () {
    return this.attributes_['volume'];
};

VpaidVideoPlayer.prototype.setAdVolume = function (value) {
    this.attributes_['volume'] = value;
    this.ads_.changeVolume(this.attributes_['volume']);
    this.callEvent_('AdVolumeChange');
};

VpaidVideoPlayer.prototype.getAdCompanions = function () {
    return this.attributes_['companions'];
};

VpaidVideoPlayer.prototype.getAdIcons = function () {
    return this.attributes_['icons'];
};

VpaidVideoPlayer.prototype.log = function (message) {
    if (this.isDebug) console.log(message);
};

VpaidVideoPlayer.prototype.callEvent_ = function (eventType) {
    this.log('EVENT' + eventType);
    if (eventType in this.eventsCallbacks_) {
        this.eventsCallbacks_[eventType]();
    }
};

VpaidVideoPlayer.prototype.loadAds_ = function () {
    this.videoPlayer_.removePreloadListener();
    this.ads_.requestAds(this.adTagUrl_, this.adTagRequestUrl_);
};

VpaidVideoPlayer.prototype.bind_ = function (thisObj, fn) {
    return function () {
        fn.apply(thisObj, arguments);
    };
};

VpaidVideoPlayer.prototype.makeHttpRequest = function (url) {
    var http = new XMLHttpRequest();
    var params = "";
    http.open("GET", url, true);

    // http.onreadystatechange = function () {
    //     if (http.readyState === 4 && http.status === 200) {
    //         console.log(http.responseText);
    //     }
    // };
    http.send(params);
};

// VpaidVideoPlayer.prototype.ajax_ = function () {
//     var http = new XMLHttpRequest();
//     var url = "/";
//     var params = "123";
//     http.open("POST", url, true);
//
//     http.setRequestHeader("Content-type", "application/x-www-form-urlencoded");
//
//     http.onreadystatechange = function () {
//         if (http.readyState === 4 && http.status === 200) {
//             console.log(http.responseText);
//         }
//     };
//     http.send(params);
// };

VpaidVideoPlayer.prototype.onContentEnded_ = function () {
    this.log('ended');
    this.ads_.contentEnded();
    this.slot_.remove();
    this.callEvent_("AdStopped");
};

VpaidVideoPlayer.prototype.setVideoEndedCallbackEnabled = function (enable) {
    if (enable) {
        this.videoPlayer_.registerVideoEndedCallback(this.videoEndedCallback_);
    } else {
        this.videoPlayer_.removeVideoEndedCallback(this.videoEndedCallback_);
    }
};
//</editor-fold>

//<editor-fold desc="Ads">
var Ads = function (application, videoPlayer) {
    this.application_ = application;
    this.videoPlayer_ = videoPlayer;
    google.ima.settings.setVpaidMode(google.ima.ImaSdkSettings.VpaidMode.ENABLED);

    this.adDisplayContainer_ = new google.ima.AdDisplayContainer(this.videoPlayer_.adContainer, this.videoPlayer_.contentPlayer);
    this.adsLoader_ = new google.ima.AdsLoader(this.adDisplayContainer_);

    this.adsManager_ = null;

    this.adsLoader_.addEventListener(google.ima.AdsManagerLoadedEvent.Type.ADS_MANAGER_LOADED, this.onAdsManagerLoaded_, false, this);
    this.adsLoader_.addEventListener(google.ima.AdErrorEvent.Type.AD_ERROR, this.onAdError_, false, this);
};

Ads.prototype.initialUserAction = function () {
    this.adDisplayContainer_.initialize();
    //console.log('%c Ads.prototype.initialUserAction', 'background: green; color: white;');
    this.videoPlayer_.contentPlayer.load();
};

Ads.prototype.requestAds = function (adTagUrl, requestUrl) {
    var adsRequest = new google.ima.AdsRequest();
    adsRequest.adTagUrl = adTagUrl;
    adsRequest.linearAdSlotWidth = this.videoPlayer_.width;
    adsRequest.linearAdSlotHeight = this.videoPlayer_.height;
    adsRequest.nonLinearAdSlotWidth = this.videoPlayer_.width;
    adsRequest.nonLinearAdSlotHeight = this.videoPlayer_.height;
    this.adsLoader_.requestAds(adsRequest);
    this.application_.makeHttpRequest(requestUrl);
};

Ads.prototype.pause = function () {
    //console.log('%c Ads pause', 'background: silver; color: white;');

    if (this.adsManager_) {
        this.adsManager_.pause();
        this.application_.callEvent_('AdPaused');
    }
};

Ads.prototype.resume = function () {
    //console.log('%c Ads resume', 'background: silver; color: white;');

    if (this.adsManager_) {
        this.adsManager_.resume();
        this.application_.callEvent_('AdPlaying');
    }
};

Ads.prototype.resize = function (width, height) {
    if (this.adsManager_) {
        console.log('%c Ads.prototype.resize', 'background: green; color: white;');
        this.adsManager_.resize(width, height, google.ima.ViewMode.FULLSCREEN);
    }
};

Ads.prototype.contentEnded = function () {
    this.adsLoader_.contentComplete();
};

Ads.prototype.onAdsManagerLoaded_ = function (adsManagerLoadedEvent) {
    this.application_.log('Ads loaded.');
    var adsRenderingSettings = new google.ima.AdsRenderingSettings();
    adsRenderingSettings.restoreCustomPlaybackStateOnAdBreakComplete = true;
    this.adsManager_ = adsManagerLoadedEvent.getAdsManager(this.videoPlayer_.contentPlayer, adsRenderingSettings);
    this.startAdsManager_(this.adsManager_);
};

Ads.prototype.startAdsManager_ = function (adsManager) {
    adsManager.addEventListener(google.ima.AdEvent.Type.CONTENT_PAUSE_REQUESTED, this.onContentPauseRequested_, false, this);
    adsManager.addEventListener(google.ima.AdEvent.Type.CONTENT_RESUME_REQUESTED, this.onContentResumeRequested_, false, this);

    var events = [google.ima.AdEvent.Type.ALL_ADS_COMPLETED,
        google.ima.AdEvent.Type.IMPRESSION,
        google.ima.AdEvent.Type.CLICK,
        google.ima.AdEvent.Type.COMPLETE,
        google.ima.AdEvent.Type.FIRST_QUARTILE,
        google.ima.AdEvent.Type.LOADED,
        google.ima.AdEvent.Type.MIDPOINT,
        google.ima.AdEvent.Type.PAUSED,
        google.ima.AdEvent.Type.RESUMED,
        google.ima.AdEvent.Type.STARTED,
        google.ima.AdEvent.Type.VOLUME_CHANGED,
        google.ima.AdEvent.Type.VOLUME_MUTED,
        google.ima.AdEvent.Type.SKIPPED,
        google.ima.AdEvent.Type.THIRD_QUARTILE];

    for (var i = 0; i < events.length; i++) {
        adsManager.addEventListener(events[i], this.onAdEvent_, false, this);
    }

    var initWidth, initHeight;
    if (this.application_.fullscreen) {
        initWidth = this.application_.fullscreenWidth;
        initHeight = this.application_.fullscreenHeight;
    } else {
        initWidth = this.application_.attributes_.width;
        initHeight = this.application_.attributes_.height;
    }
    adsManager.init(initWidth, initHeight, google.ima.ViewMode.NORMAL);
    adsManager.start();
};

Ads.prototype.onContentPauseRequested_ = function () {
    if (this.application_.isDebug) console.log('%c onContentPauseRequested_', 'background: red; color: white;');
    //TODO WHY???
    // this.pause();
    this.application_.setVideoEndedCallbackEnabled(false);
};

Ads.prototype.onContentResumeRequested_ = function () {
    if (this.application_.isDebug) console.log('%c onContentResumeRequested_', 'background: red; color: white;');
    //TODO WHY???
    // this.resume();
    this.application_.setVideoEndedCallbackEnabled(true);
};

Ads.prototype.onAdEvent_ = function (adEvent) {
    this.application_.log('Ad event: ' + adEvent.type);

    if (adEvent.type === google.ima.AdEvent.Type.IMPRESSION) {
        if (this.application_.isDebug) console.log('IMPRESSION');
        this.application_.makeHttpRequest(this.application_.adTagImpressionUrl_);
    } else if (adEvent.type === google.ima.AdEvent.Type.ALL_ADS_COMPLETED) {
        this.onAdError_();

        if (this.adsManager_) {
            this.adsManager_.destroy();
        }
    } else if (adEvent.type === google.ima.AdEvent.Type.LOADED) {
        if (this.application_.isDebug) console.log('%c AD LOADED ', 'background: green; color: white; display: block;');
        var ad = adEvent.getAd();
        if (!ad.isLinear()) {
            this.onContentResumeRequested_();
        }
    } else if (adEvent.type === google.ima.AdEvent.Type.SKIPPED) {
        this.application_.callEvent_('AdSkipped');
    } else if (adEvent.type === google.ima.AdEvent.Type.PAUSED) {
        this.application_.callEvent_('AdPaused');
    } else if (adEvent.type === google.ima.AdEvent.Type.RESUMED) {
        this.application_.callEvent_('AdPlaying');
    }
};

Ads.prototype.onAdError_ = function (adErrorEvent) {
    this.application_.adIndex++;
    this.application_.startWaterfall_();
    if (this.adsManager_) {
        this.adsManager_.destroy();
    }
};

Ads.prototype.changeVolume = function (volume) {
    this.adsManager_.setVolume(volume);
};
//</editor-fold>

//<editor-fold desc="Video Player">
var VideoPlayer = function (width, height) {
    this.contentPlayer = document.getElementById('content');
    this.adContainer = document.getElementById('adcontainer');
    this.videoPlayerContainer_ = document.getElementById('videoplayer');

    this.width = width;
    this.height = height;
};

VideoPlayer.prototype.preloadContent = function (contentLoadedAction) {
    contentLoadedAction();
    // if (this.isMobilePlatform()) {
    //     this.preloadListener_ = contentLoadedAction;
    //     this.contentPlayer.addEventListener('loadedmetadata', contentLoadedAction, false);
    //     this.contentPlayer.load();
    // } else {
    //     contentLoadedAction();
    // }
};

VideoPlayer.prototype.removePreloadListener = function () {
    if (this.preloadListener_) {
        this.contentPlayer.removeEventListener('loadedmetadata', this.preloadListener_, false);
        this.preloadListener_ = null;
    }
};

VideoPlayer.prototype.play = function() {
    this.contentPlayer.style.display = 'block';
    this.contentPlayer.play();
};

VideoPlayer.prototype.pause = function() {
    this.contentPlayer.pause();
};

VideoPlayer.prototype.isMobilePlatform = function () {
    return this.contentPlayer.paused && (navigator.userAgent.match(/(iPod|iPhone|iPad)/) || navigator.userAgent.toLowerCase().indexOf('android') > -1);
};

VideoPlayer.prototype.resize = function (position, top, left, width, height) {
    this.videoPlayerContainer_.style.position = position;
    this.videoPlayerContainer_.style.top = top + 'px';
    this.videoPlayerContainer_.style.left = left + 'px';
    this.videoPlayerContainer_.style.width = width + 'px';
    this.videoPlayerContainer_.style.height = height + 'px';
    this.contentPlayer.style.width = width + 'px';
    this.contentPlayer.style.height = height + 'px';
};

VideoPlayer.prototype.registerVideoEndedCallback = function (callback) {
    this.contentPlayer.addEventListener('ended', callback, false);
};

VideoPlayer.prototype.removeVideoEndedCallback = function (callback) {
    this.contentPlayer.removeEventListener('ended', callback, false);
};
//</editor-fold>
