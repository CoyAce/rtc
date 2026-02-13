// SPDX-License-Identifier: Unlicense OR MIT

//go:build ios

#import <PhotosUI/PhotosUI.h>
#include <stdint.h>
#include "_cgo_export.h"

@implementation photo_picker
- (void)pickPhotos {
    if (@available(iOS 14, *)) {
        PHPickerConfiguration *config = [[PHPickerConfiguration alloc] init];
        config.selectionLimit = 1;
        config.filter = [PHPickerFilter imagesFilter];

        PHPickerViewController *picker = [[PHPickerViewController alloc]
            initWithConfiguration:config];
        picker.delegate = self;

        [self.controller presentViewController:picker
                                      animated:YES
                                    completion:nil];
    }
}

- (void)picker:(PHPickerViewController *)picker
    didFinishPicking:(NSArray<PHPickerResult *> *)results {

    [picker dismissViewControllerAnimated:YES completion:nil];

    if (results.count == 0) {
        // 用户取消了选择
        importPhoto(NULL);
        return;
    }

    PHPickerResult *result = [results firstObject];
    NSItemProvider *provider = result.itemProvider;

    if ([provider canLoadObjectOfClass:[UIImage class]]) {
        [provider loadObjectOfClass:[UIImage class]
                  completionHandler:^(id<NSItemProviderReading> object,
                                      NSError *error) {
            if (error) {
                importPhoto(NULL);
                return;
            }

            UIImage *image = (UIImage *)object;

            // 保存到 Documents
            NSData *imageData = UIImageJPEGRepresentation(image, 0.8);
            NSString *fileName = [NSString stringWithFormat:@"%@.jpeg",
                [NSUUID UUID].UUIDString];
            NSString *docDir = NSSearchPathForDirectoriesInDomains(
                NSDocumentDirectory, NSUserDomainMask, YES).firstObject;
            NSString *filePath = [docDir stringByAppendingPathComponent:fileName];
            NSURL *fileURL = [NSURL fileURLWithPath:filePath];

            [imageData writeToURL:fileURL
                          options:NSDataWritingAtomic
                            error:nil];

            // 回调 Go 层
            importPhoto(strdup([fileURL.path UTF8String]));
        }];
    }
}
@end

CFTypeRef createPhotoPicker(CFTypeRef controllerRef) {
	photo_picker *p = [[photo_picker alloc] init];
	p.controller = (__bridge UIViewController *)controllerRef;
	return (__bridge_retained CFTypeRef)p;
}

void pickPhotos(CFTypeRef pickerRef) {
    photo_picker *picker = (__bridge photo_picker *)pickerRef;
    [picker pickPhotos];
}