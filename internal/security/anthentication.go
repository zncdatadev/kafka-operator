package security

// type AuthenticationProvider []func() (interface{}, error)

// func (a AuthenticationProvider) Register(f func() (interface{}, error)) {
// 	a = append(a, f)
// }

// pub async fn from_references(
//     client: &Client,
//     auth_classes: &Vec<KafkaAuthentication>,
// ) -> Result<ResolvedAuthenticationClasses, Error> {
//     let mut resolved_authentication_classes: Vec<AuthenticationClass> = vec![];

//     for auth_class in auth_classes {
//         resolved_authentication_classes.push(
//             AuthenticationClass::resolve(client, &auth_class.authentication_class)
//                 .await
//                 .context(AuthenticationClassRetrievalSnafu {
//                     authentication_class: ObjectRef::<AuthenticationClass>::new(
//                         &auth_class.authentication_class,
//                     ),
//                 })?,
//         );
//     }

//	    ResolvedAuthenticationClasses::new(resolved_authentication_classes).validate()
//	}
// func ResolverAuthenticationClass(
// 	client *client.Client,
// 	kafkaAuthClasses []kafkav1alpha1.KafkaAuthenticationSpec,
// ) (interface{}, error) {
// 	// todo: resolve authentication classes with client, currently, kerberos use inline provider
// 	for _, authClass := range kafkaAuthClasses {
// 		// resolve each authentication class with the client

// 	}
// 	return nil, nil
// }
