// SPDX-License-Identifier: Unlicense OR MIT

//go:build ios

#import <PhotosUI/PhotosUI.h>
#import <Photos/Photos.h>
#import <ImageIO/ImageIO.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdint.h>
#include "_cgo_export.h"

NSString* getDocumentsDirectory(void);

@implementation photo_picker
- (void)pickPhotos {
    if (@available(iOS 14, *)) {
        PHPickerConfiguration *config = [[PHPickerConfiguration alloc] init];
        config.selectionLimit = 1;
        config.filter = [PHPickerFilter imagesFilter];

        PHPickerViewController *picker = [[PHPickerViewController alloc]
            initWithConfiguration:config];
        picker.delegate = self;

        [self.controller presentViewController:picker animated:YES completion:nil];
    }
}

- (void)picker:(PHPickerViewController *)picker
    didFinishPicking:(NSArray<PHPickerResult *> *)results {

    [picker dismissViewControllerAnimated:YES completion:nil];

    if (results.count == 0) {
        importPhoto(NULL);
        return;
    }

    PHPickerResult *result = [results firstObject];
    NSItemProvider *provider = result.itemProvider;

     [provider loadFileRepresentationForTypeIdentifier:UTTypeImage.identifier
                                        completionHandler:^(NSURL *url, NSError *error) {
        if (error || !url) {
            NSLog(@"load file failed: %@", error);
            importPhoto(NULL);
            return;
        }
        [self handleFileURL:url];
    }];
}

- (void)handleFileURL:(NSURL *)url {
    [url startAccessingSecurityScopedResource];

    NSString *docDir = getDocumentsDirectory();
    NSString *destPath = [docDir stringByAppendingPathComponent: url.lastPathComponent];
    NSURL *destURL = [NSURL fileURLWithPath:destPath];

    NSError *error = nil;
    if (![[NSFileManager defaultManager] fileExistsAtPath:destPath]) {
        [[NSFileManager defaultManager] copyItemAtURL:url toURL:destURL error:&error];
    }

    [url stopAccessingSecurityScopedResource];

    dispatch_async(dispatch_get_main_queue(), ^{
        if (!error) {
            importPhoto(strdup([destPath UTF8String]));
        } else {
            NSLog(@"copy file failed: %@", error);
            importPhoto(NULL);
        }
    });
}

- (void)savePhoto:(NSString *)path {
    NSFileManager *fm = [NSFileManager defaultManager];
    if (![fm fileExistsAtPath:path]) {
        exportPhoto(strdup("file not found"));
        return;
    }

    UIImage *image = [UIImage imageWithContentsOfFile:path];
    if (!image) {
        exportPhoto(strdup("failed to load image"));
        return;
    }

    [PHPhotoLibrary requestAuthorization:^(PHAuthorizationStatus status) {
        if (status != PHAuthorizationStatusAuthorized) {
            exportPhoto(strdup("no photo library permission"));
            return;
        }

        [[PHPhotoLibrary sharedPhotoLibrary] performChanges:^{
            [PHAssetChangeRequest creationRequestForAssetFromImage:image];
        } completionHandler:^(BOOL success, NSError *error) {
            if (success) {
                exportPhoto(NULL);  // 成功时传 NULL
                return;
            }

            if (error) {
                if ([error.domain isEqualToString:PHPhotosErrorDomain]) {
                    switch (error.code) {
                        case PHPhotosErrorUserCancelled:
                            exportPhoto(strdup("user cancelled"));
                            break;
                        case PHPhotosErrorAccessUserDenied:
                            exportPhoto(strdup("access denied"));
                            break;
                        default:
                            exportPhoto(strdup("save failed"));
                            break;
                    }
                }
                return;
            }
            exportPhoto(strdup("save failed"));
        }];
    }];
}
@end

CFTypeRef createPhotoPicker(CFTypeRef controllerRef) {
	photo_picker *p = [[photo_picker alloc] init];
	p.controller = (__bridge UIViewController *)controllerRef;
	return (__bridge_retained CFTypeRef)p;
}

void pickPhoto(CFTypeRef pickerRef) {
    photo_picker *picker = (__bridge photo_picker *)pickerRef;
    [picker pickPhotos];
}

void savePhoto(CFTypeRef pickerRef, const char* path) {
    photo_picker *picker = (__bridge photo_picker *)pickerRef;
    NSString *p = [NSString stringWithUTF8String:path];
    [picker savePhoto:p];
}

NSString* getDocumentsDirectory(void) {
    NSArray *paths = NSSearchPathForDirectoriesInDomains(
        NSDocumentDirectory,
        NSUserDomainMask,
        YES
    );
    return paths.firstObject;
}

const char* getDocDir(void) {
    NSString *docDir = getDocumentsDirectory();
    return strdup([docDir UTF8String]);
}