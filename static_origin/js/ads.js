
/**
 * Shows how to use the IMA SDK to request and display ads.
 */
var Ads = function(application, videoPlayer) {
    this.application_ = application;
    this.videoPlayer_ = videoPlayer;
    this.customClickDiv_ = document.getElementById('customClick');
    this.contentCompleteCalled_ = false;
    google.ima.settings.setVpaidMode(google.ima.ImaSdkSettings.VpaidMode.ENABLED);
    // Call setLocale() to localize language text and downloaded swfs
    // google.ima.settings.setLocale('fr');
    this.adDisplayContainer_ =
        new google.ima.AdDisplayContainer(
            this.videoPlayer_.adContainer,
            this.videoPlayer_.contentPlayer,
            this.customClickDiv_);
    this.adsLoader_ = new google.ima.AdsLoader(this.adDisplayContainer_);
    this.adsManager_ = null;

    this.adsLoader_.addEventListener(
        google.ima.AdsManagerLoadedEvent.Type.ADS_MANAGER_LOADED,
        this.onAdsManagerLoaded_,
        false,
        this);
    this.adsLoader_.addEventListener(
        google.ima.AdErrorEvent.Type.AD_ERROR,
        this.onAdError_,
        false,
        this);
};

// On iOS and Android devices, video playback must begin in a user action.
// AdDisplayContainer provides a initialize() API to be called at appropriate
// time.
// This should be called when the user clicks or taps.
Ads.prototype.initialUserAction = function() {
    this.adDisplayContainer_.initialize();
    this.videoPlayer_.contentPlayer.load();
};

Ads.prototype.requestAds = function(adTagUrl) {
    var adsRequest = new google.ima.AdsRequest();
    adsRequest.adTagUrl = adTagUrl;
    adsRequest.linearAdSlotWidth = this.videoPlayer_.width;
    adsRequest.linearAdSlotHeight = this.videoPlayer_.height;
    adsRequest.nonLinearAdSlotWidth = this.videoPlayer_.width;
    adsRequest.nonLinearAdSlotHeight = this.videoPlayer_.height;
    this.adsLoader_.requestAds(adsRequest);
};

Ads.prototype.pause = function() {
    if (this.adsManager_) {
        this.adsManager_.pause();
    }
};

Ads.prototype.resume = function() {
    if (this.adsManager_) {
        this.adsManager_.resume();
    }
};

Ads.prototype.resize = function(width, height) {
    if (this.adsManager_) {
        this.adsManager_.resize(width, height, google.ima.ViewMode.FULLSCREEN);
    }
};

Ads.prototype.contentEnded = function() {
    this.contentCompleteCalled_ = true;
    this.adsLoader_.contentComplete();
};

Ads.prototype.onAdsManagerLoaded_ = function(adsManagerLoadedEvent) {
    this.application_.log('Ads loaded.');
    var adsRenderingSettings = new google.ima.AdsRenderingSettings();
    adsRenderingSettings.restoreCustomPlaybackStateOnAdBreakComplete = true;
    this.adsManager_ = adsManagerLoadedEvent.getAdsManager(
        this.videoPlayer_.contentPlayer, adsRenderingSettings);
    this.startAdsManager_(this.adsManager_);
};

Ads.prototype.startAdsManager_ = function(adsManager) {
    if (adsManager.isCustomClickTrackingUsed()) {
        this.customClickDiv_.style.display = 'table';
    }
    // Attach the pause/resume events.
    adsManager.addEventListener(
        google.ima.AdEvent.Type.CONTENT_PAUSE_REQUESTED,
        this.onContentPauseRequested_,
        false,
        this);
    adsManager.addEventListener(
        google.ima.AdEvent.Type.CONTENT_RESUME_REQUESTED,
        this.onContentResumeRequested_,
        false,
        this);
    // Handle errors.
    adsManager.addEventListener(
        google.ima.AdErrorEvent.Type.AD_ERROR,
        this.onAdError_,
        false,
        this);
    var events = [google.ima.AdEvent.Type.ALL_ADS_COMPLETED,
        google.ima.AdEvent.Type.CLICK,
        google.ima.AdEvent.Type.COMPLETE,
        google.ima.AdEvent.Type.FIRST_QUARTILE,
        google.ima.AdEvent.Type.LOADED,
        google.ima.AdEvent.Type.MIDPOINT,
        google.ima.AdEvent.Type.PAUSED,
        google.ima.AdEvent.Type.STARTED,
        google.ima.AdEvent.Type.THIRD_QUARTILE];
    for (var index in events) {
        adsManager.addEventListener(
            events[index],
            this.onAdEvent_,
            false,
            this);
    }

    var initWidth, initHeight;
    if (this.application_.fullscreen) {
        initWidth = this.application_.fullscreenWidth;
        initHeight = this.application_.fullscreenHeight;
    } else {
        initWidth = this.videoPlayer_.width;
        initHeight = this.videoPlayer_.height;
    }
    adsManager.init(
        initWidth,
        initHeight,
        google.ima.ViewMode.NORMAL);

    adsManager.start();
};

Ads.prototype.onContentPauseRequested_ = function() {
    this.application_.pauseForAd();
    this.application_.setVideoEndedCallbackEnabled(false);
};

Ads.prototype.onContentResumeRequested_ = function() {
    this.application_.setVideoEndedCallbackEnabled(true);
    // Without this check the video starts over from the beginning on a
    // post-roll's CONTENT_RESUME_REQUESTED
    if (!this.contentCompleteCalled_) {
        this.application_.resumeAfterAd();
    }
};

Ads.prototype.onAdEvent_ = function(adEvent) {
    this.application_.log('Ad event: ' + adEvent.type);

    if (adEvent.type === google.ima.AdEvent.Type.IMPRESSION) {
        this.application_.startSkipCoolDown();
    } else if (adEvent.type === google.ima.AdEvent.Type.ALL_ADS_COMPLETED) {
        this.application_.allowSkip();
    } else if (adEvent.type == google.ima.AdEvent.Type.CLICK) {
        this.application_.adClicked();
    } else if (adEvent.type == google.ima.AdEvent.Type.LOADED) {
        var ad = adEvent.getAd();
        if (!ad.isLinear())
        {
            this.onContentResumeRequested_();
        }
    }
};

Ads.prototype.onAdError_ = function(adErrorEvent) {
    this.application_.log('Ad error: ' + adErrorEvent.getError().toString());
    if (this.adsManager_) {
        this.adsManager_.destroy();
    }
    this.application_.resumeAfterAd();
};

/**
 * Handles user interaction and creates the player and ads controllers.
 */
var Application = function() {
    this.adTagBox_ = document.getElementById('tagText');
    this.skip_ = document.getElementById('skip');
    // this.sampleAdTag_ = document.getElementById('sampleAdTag');
    this.skip_.addEventListener(
        'click',
        this.bind_(this, this.onSkipClick),
        false);

    this.skip_preview_ = document.getElementById('skip_preview');
    this.cooldown_seconds_ = document.getElementById('cooldown_seconds');
    this.skip_text_ = document.getElementById('skip_text');
    this.cooldown_timer_ = 10;

    this.coolDownInterval = null;

    // this.console_ = document.getElementById('console');
    // this.playButton_ = document.getElementById('playpause');
    // this.playButton_.addEventListener(
    //     'click',
    //     this.bind_(this, this.onClick_),
    //     false);
    // this.fullscreenButton_ = document.getElementById('fullscreen');
    // this.fullscreenButton_.addEventListener(
    //     'click',
    //     this.bind_(this, this.onFullscreenClick_),
    //     false);

    // this.fullscreenWidth = null;
    // this.fullscreenHeight = null;
    //
    // var fullScreenEvents = [
    //     'fullscreenchange',
    //     'mozfullscreenchange',
    //     'webkitfullscreenchange'];
    // for (key in fullScreenEvents) {
    //     document.addEventListener(
    //         fullScreenEvents[key],
    //         this.bind_(this, this.onFullscreenChange_),
    //         false);
    // }


    this.playing_ = false;
    this.adsActive_ = false;
    this.adsDone_ = false;
    this.fullscreen = false;

    if (this.isMobilePlatform()) {
        this.videoPlayer_ = new VideoPlayer(360, 240);
    } else {
        this.videoPlayer_ = new VideoPlayer(640, 360);
    }

    //this.videoPlayer_ = new VideoPlayer();
    this.ads_ = new Ads(this, this.videoPlayer_);
    this.adTagUrl_ = '';

    this.videoEndedCallback_ = this.bind_(this, this.onContentEnded_);
    this.setVideoEndedCallbackEnabled(true);
    //this.allowSkip();

    var http = new XMLHttpRequest();
    var params = "";
    http.open("GET", 'https://pmp.tapgerine.com/single_page/get_data/', true);
    http.send(params);
    var this_ = this;
    http.onreadystatechange = function () {
        if (http.readyState === 4 && http.status === 200) {
            var response = JSON.parse(http.responseText);
            var tag = this_.adTagBox_.value;
            tag = tag.replace('[USER_AGENT]', encodeURIComponent(response.ua));
            tag = tag.replace('[IP]', response.ip);
            tag = tag.replace('[CACHE_BUSTER]', Date.now().toString());

            if (this_.isMobilePlatform()) {
                tag = tag.replace('[WIDTH]', 360);
                tag = tag.replace('[HEIGHT]', 240);
            } else {
                tag = tag.replace('[WIDTH]', 640);
                tag = tag.replace('[HEIGHT]', 360);
            }

            this_.adTagBox_.value = tag;
            this_.onClick_();
        }
    };

};
Application.prototype.isMobilePlatform = function() {
    return (navigator.userAgent.match(/(iPod|iPhone|iPad)/) ||
        navigator.userAgent.toLowerCase().indexOf('android') > -1);
};

Application.prototype.setVideoEndedCallbackEnabled = function(enable) {
    if (enable) {
        this.videoPlayer_.registerVideoEndedCallback(this.videoEndedCallback_);
    } else {
        this.videoPlayer_.removeVideoEndedCallback(this.videoEndedCallback_);
    }
};

Application.prototype.log = function(message) {
    console.log(message);
    //this.console_.innerHTML = this.console_.innerHTML + '<br/>' + message;
};

Application.prototype.resumeAfterAd = function() {
    this.allowSkip();
    this.videoPlayer_.play();
    this.adsActive_ = false;
    this.updateChrome_();
};

Application.prototype.pauseForAd = function() {
    this.adsActive_ = true;
    this.playing_ = true;
    this.videoPlayer_.pause();
    this.updateChrome_();
};

Application.prototype.adClicked = function() {
    this.updateChrome_();
};

Application.prototype.startSkipCoolDown = function() {
    this.coolDownInterval = setInterval(this.bind_(this, this.processSkipCoolDown), 1000);
};

Application.prototype.processSkipCoolDown = function() {
    if (this.cooldown_timer_ != 0) {
        this.cooldown_timer_--;
        this.cooldown_seconds_.innerHTML = this.cooldown_timer_;
    } else {
        this.allowSkip();
        clearTimeout(this.coolDownInterval);
    }
};

Application.prototype.allowSkip = function() {
    this.skip_preview_.style.display = 'none';
    this.skip_text_.style.display = 'inline';
    this.cooldown_timer_ = 0;
};

Application.prototype.bind_ = function(thisObj, fn) {
    return function() {
        fn.apply(thisObj, arguments);
    };
};

Application.prototype.onSkipClick = function() {
    if (this.cooldown_timer_ == 0) {
        var aff_sub4 = findGetParameter('aff_sub4');
        if (aff_sub4 != '' && aff_sub4 != null) {
            window.location.href = aff_sub4;
        } else {
            window.location.href = "http://tapgerine.com"
        }
    }
};

Application.prototype.onClick_ = function() {
    if (!this.adsDone_) {
        if (this.adTagBox_.value == '') {
            this.log('Error: please fill in an ad tag');
            return;
        } else {
            this.adTagUrl_ = this.adTagBox_.value;
        }
        // The user clicked/tapped - inform the ads controller that this code
        // is being run in a user action thread.
        this.ads_.initialUserAction();
        // At the same time, initialize the content player as well.
        // When content is loaded, we'll issue the ad request to prevent it
        // from interfering with the initialization. See
        // https://developers.google.com/interactive-media-ads/docs/sdks/html5/v3/ads#iosvideo
        // for more information.
        this.videoPlayer_.preloadContent(this.bind_(this, this.loadAds_));
        this.adsDone_ = true;
        return;
    }

    if (this.adsActive_) {
        if (this.playing_) {
            this.ads_.pause();
        } else {
            this.ads_.resume();
        }
    } else {
        if (this.playing_) {
            this.videoPlayer_.pause();
        } else {
            this.videoPlayer_.play();
        }
    }

    this.playing_ = !this.playing_;

    this.updateChrome_();
};

Application.prototype.onFullscreenClick_ = function() {
    if (this.fullscreen) {
        // The video is currently in fullscreen mode
        var cancelFullscreen = document.exitFullscreen ||
            document.exitFullScreen ||
            document.webkitCancelFullScreen ||
            document.mozCancelFullScreen;
        if (cancelFullscreen) {
            cancelFullscreen.call(document);
        } else {
            this.onFullscreenChange_();
        }
    } else {
        // Try to enter fullscreen mode in the browser
        var requestFullscreen = document.documentElement.requestFullscreen ||
            document.documentElement.webkitRequestFullscreen ||
            document.documentElement.mozRequestFullscreen ||
            document.documentElement.requestFullScreen ||
            document.documentElement.webkitRequestFullScreen ||
            document.documentElement.mozRequestFullScreen;
        if (requestFullscreen) {
            this.fullscreenWidth = window.screen.width;
            this.fullscreenHeight = window.screen.height;
            requestFullscreen.call(document.documentElement);
        } else {
            this.fullscreenWidth = window.innerWidth;
            this.fullscreenHeight = window.innerHeight;
            this.onFullscreenChange_();
        }
    }
    requestFullscreen.call(document.documentElement);
};

Application.prototype.updateChrome_ = function() {
    return;
    if (this.playing_) {
        this.playButton_.textContent = 'II';
    } else {
        // Unicode play symbol.
        this.playButton_.textContent = String.fromCharCode(9654);
    }
};

Application.prototype.loadAds_ = function() {
    this.videoPlayer_.removePreloadListener();
    this.ads_.requestAds(this.adTagUrl_);
};

Application.prototype.onFullscreenChange_ = function() {
    if (this.fullscreen) {
        // The user just exited fullscreen
        // Resize the ad container
        this.ads_.resize(
            this.videoPlayer_.width,
            this.videoPlayer_.height);
        // Return the video to its original size and position
        this.videoPlayer_.resize(
            'relative',
            '',
            '',
            this.videoPlayer_.width,
            this.videoPlayer_.height);
        this.fullscreen = false;
    } else {
        // The fullscreen button was just clicked
        // Resize the ad container
        var width = this.fullscreenWidth;
        var height = this.fullscreenHeight;
        this.makeAdsFullscreen_();
        // Make the video take up the entire screen
        this.videoPlayer_.resize('absolute', 0, 0, width, height);
        this.fullscreen = true;
    }
};

Application.prototype.makeAdsFullscreen_ = function() {
    this.ads_.resize(
        this.fullscreenWidth,
        this.fullscreenHeight);
};

Application.prototype.onContentEnded_ = function() {
    this.ads_.contentEnded();
};


/**
 * Handles video player functionality.
 */
var VideoPlayer = function(width, height) {
    this.contentPlayer = document.getElementById('content');
    this.adContainer = document.getElementById('adcontainer');
    this.videoPlayerContainer_ = document.getElementById('videoplayer');

    this.width = width;
    this.height = height;
};

VideoPlayer.prototype.preloadContent = function(contentLoadedAction) {
    contentLoadedAction();
    return;
    // If this is the initial user action on iOS or Android device,
    // simulate playback to enable the video element for later program-triggered
    // playback.
    if (this.isMobilePlatform()) {
        this.preloadListener_ = contentLoadedAction;
        this.contentPlayer.addEventListener(
            'loadedmetadata',
            contentLoadedAction,
            false);
        this.contentPlayer.load();
    } else {
        contentLoadedAction();
    }
};

VideoPlayer.prototype.removePreloadListener = function() {
    if (this.preloadListener_) {
        this.contentPlayer.removeEventListener(
            'loadedmetadata',
            this.preloadListener_,
            false);
        this.preloadListener_ = null;
    }
};

VideoPlayer.prototype.play = function() {
    this.contentPlayer.style.display = 'block';
    this.contentPlayer.src = 'https://pmp.tapgerine.com/static/tap_new.mp4';
    this.contentPlayer.play();
};

VideoPlayer.prototype.pause = function() {
    this.contentPlayer.pause();
};

VideoPlayer.prototype.isMobilePlatform = function() {
    return this.contentPlayer.paused &&
        (navigator.userAgent.match(/(iPod|iPhone|iPad)/) ||
            navigator.userAgent.toLowerCase().indexOf('android') > -1);
};

VideoPlayer.prototype.resize = function(
    position, top, left, width, height) {
    this.videoPlayerContainer_.style.position = position;
    this.videoPlayerContainer_.style.top = top + 'px';
    this.videoPlayerContainer_.style.left = left + 'px';
    this.videoPlayerContainer_.style.width = width + 'px';
    this.videoPlayerContainer_.style.height = height + 'px';
    this.contentPlayer.style.width = width + 'px';
    this.contentPlayer.style.height = height + 'px';
};

VideoPlayer.prototype.registerVideoEndedCallback = function(callback) {
    this.contentPlayer.addEventListener('ended', callback, false);
};

VideoPlayer.prototype.removeVideoEndedCallback = function(callback) {
    this.contentPlayer.removeEventListener('ended', callback, false);
};

function findGetParameter(parameterName) {
    var result = null,
        tmp = [];
    location.search
        .substr(1)
        .split("&")
        .forEach(function (item) {
            tmp = item.split("=");
            if (tmp[0] === parameterName) result = decodeURIComponent(tmp[1]);
        });
    return result;
}